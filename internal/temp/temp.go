package temp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

type Sensor struct {
	Name string
	Path string
}

var cpugpuRE = regexp.MustCompile(`(?i)k10temp|coretemp|amdgpu|nvidia|nouveau|zenpower|x86_pkg_temp|cpu_thermal|acpitz|intel.*temp|thermal_zone`)

type Watcher struct {
	cfg     config.Config
	sensors []Sensor
	maxC    float64
	critHit bool
	warnHit bool
}

func NewWatcher(cfg config.Config) *Watcher {
	w := &Watcher{cfg: cfg}
	w.sensors = ScanSensors(cfg.Temperature.Sensors)
	return w
}

func (w *Watcher) Name() string { return "temperature" }

// ScanSensors returns matching temperature sensors from hwmon and thermal
// zones. With a non-empty filter only those exact hwmon names are kept,
// otherwise CPU/GPU sensors are auto-detected.
func ScanSensors(filter []string) []Sensor {
	var out []Sensor
	hwmonDirs, _ := filepath.Glob("/sys/class/hwmon/hwmon*")
	for _, dir := range hwmonDirs {
		name, err := os.ReadFile(filepath.Join(dir, "name"))
		if err != nil {
			continue
		}
		n := strings.TrimSpace(string(name))
		if !match(n, filter) {
			continue
		}
		inputs, _ := filepath.Glob(filepath.Join(dir, "temp*_input"))
		for _, in := range inputs {
			out = append(out, Sensor{Name: n, Path: in})
		}
	}
	zones, _ := filepath.Glob("/sys/class/thermal/thermal_zone*")
	for _, z := range zones {
		typ, err := os.ReadFile(filepath.Join(z, "type"))
		if err != nil {
			continue
		}
		if match(strings.TrimSpace(string(typ)), filter) {
			out = append(out, Sensor{Name: strings.TrimSpace(string(typ)), Path: filepath.Join(z, "temp")})
		}
	}
	return out
}

func match(name string, filter []string) bool {
	if len(filter) > 0 {
		for _, f := range filter {
			if f == name {
				return true
			}
		}
		return false
	}
	return cpugpuRE.MatchString(name)
}

// MaxTemp returns the current max CPU/GPU temperature via the watcher's
// configured sensor set.
func (w *Watcher) MaxTemp() (float64, string) {
	m, where, err := Max(w.sensors)
	if err != nil {
		return 0, ""
	}
	return m, where
}

// Max returns the highest temperature (in °C) among the sensors.
func Max(sensors []Sensor) (float64, string, error) {
	if len(sensors) == 0 {
		return 0, "", fmt.Errorf("no temperature sensors")
	}
	max := -1.0
	where := ""
	for _, s := range sensors {
		b, err := os.ReadFile(s.Path)
		if err != nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
		if err != nil {
			continue
		}
		c := v / 1000
		if c > max {
			max = c
			where = s.Name
		}
	}
	if max < 0 {
		return 0, "", fmt.Errorf("no readable sensors")
	}
	return max, where, nil
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			max, where, err := Max(w.sensors)
			if err != nil {
				continue
			}
			w.maxC = max
			crit := w.cfg.Temperature.Critical
			warn := w.cfg.Temperature.Warning

			if crit > 0 && max >= crit && !w.critHit {
				w.critHit = true
				watcher.Emit(emit, watcher.NewEvent(watcher.Critical, "temperature",
					fmt.Sprintf("%s at %.0f°C — thermal kill threshold (%.0f°C)", where, max, crit),
					map[string]float64{"temp_c": max}))
			}
			if crit > 0 && max < crit-3 {
				w.critHit = false
			}
			if warn > 0 && max >= warn && !w.critHit && !w.warnHit {
				w.warnHit = true
				watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "temperature",
					fmt.Sprintf("%s at %.0f°C — above warning threshold (%.0f°C)", where, max, warn),
					map[string]float64{"temp_c": max}))
			}
			if warn > 0 && max < warn-3 {
				w.warnHit = false
			}
		}
	}
}

func (w *Watcher) Snapshot() map[string]float64 {
	return map[string]float64{"temp_max_c": w.maxC}
}
