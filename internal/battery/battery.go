package battery

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

type Watcher struct {
	cfg       config.Config
	dir       string
	batteries []string
	enabled   bool
	capacity  float64
	lowHit    bool
}

func NewWatcher(cfg config.Config, dir string) *Watcher {
	w := &Watcher{cfg: cfg, dir: dir}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return w
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "BAT") {
			w.batteries = append(w.batteries, filepath.Join(dir, e.Name()))
		}
	}
	w.enabled = len(w.batteries) > 0
	return w
}

func (w *Watcher) Enabled() bool { return w.enabled }

func (w *Watcher) Name() string { return "battery" }

func (w *Watcher) Snapshot() map[string]float64 {
	v := 0.0
	if w.enabled {
		v = w.capacity
	}
	return map[string]float64{"battery_pct": v, "battery_enabled": float64(boolToInt(w.enabled))}
}

func (w *Watcher) read() (pct float64, status string, err error) {
	b, err := os.ReadFile(filepath.Join(w.batteries[0], "capacity"))
	if err != nil {
		return 0, "", err
	}
	pct, err = strconv.ParseFloat(strings.TrimSpace(string(b)), 64)
	if err != nil {
		return 0, "", err
	}
	s, err := os.ReadFile(filepath.Join(w.batteries[0], "status"))
	if err != nil {
		return pct, "", err
	}
	return pct, strings.TrimSpace(string(s)), nil
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	if !w.enabled {
		return nil
	}
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			pct, status, err := w.read()
			if err != nil {
				continue
			}
			w.capacity = pct
			if status == "Discharging" && pct <= w.cfg.Battery.LowPct && !w.lowHit {
				w.lowHit = true
				watcher.Emit(emit, watcher.NewEvent(watcher.Critical, "battery",
					fmt.Sprintf("battery at %.0f%% — suspending heavy processes", pct),
					map[string]float64{"battery_pct": pct}))
			} else if status != "Discharging" && w.lowHit {
				w.lowHit = false
				watcher.Emit(emit, watcher.NewEvent(watcher.Info, "battery_resume",
					fmt.Sprintf("battery charging (%.0f%%) — resuming suspended processes", pct),
					map[string]float64{"battery_pct": pct}))
			}
		}
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
