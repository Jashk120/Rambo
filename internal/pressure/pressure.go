package pressure

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

type PSI struct {
	Some10, Some60, Some300 float64
	Full10, Full60, Full300 float64
	Total                   float64
}

func Read(kind string) (PSI, error) {
	var p PSI
	b, err := os.ReadFile("/proc/pressure/" + kind)
	if err != nil {
		return p, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		vals := map[string]float64{}
		for _, f := range fields[1:] {
			kv := strings.SplitN(f, "=", 2)
			if len(kv) == 2 {
				vals[kv[0]], _ = strconv.ParseFloat(kv[1], 64)
			}
		}
		switch fields[0] {
		case "some":
			p.Some10 = vals["avg10"]
			p.Some60 = vals["avg60"]
			p.Some300 = vals["avg300"]
			p.Total = vals["total"]
		case "full":
			p.Full10 = vals["avg10"]
			p.Full60 = vals["avg60"]
			p.Full300 = vals["avg300"]
		}
	}
	return p, nil
}

type Watcher struct {
	cfg config.Config

	someStreak int
	fullStreak int
	someHit    bool
	fullHit    bool

	some10 float64
	full10 float64
	cpu10  float64
	io10   float64
	ioFull float64
}

func NewWatcher(cfg config.Config) *Watcher {
	return &Watcher{cfg: cfg}
}

func (w *Watcher) Name() string { return "memory_pressure" }

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	window := 10 * time.Second
	if d, err := time.ParseDuration(w.cfg.MemoryPressure.Window); err == nil && d > 0 {
		window = d
	}
	windowSec := int(window.Seconds())
	if windowSec < 1 {
		windowSec = 1
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			mem, err := Read("memory")
			if err == nil {
				w.some10 = mem.Some10
				w.full10 = mem.Full10

				mp := w.cfg.MemoryPressure
				if mp.SomePct > 0 && mem.Some10 > mp.SomePct {
					w.someStreak++
					if w.someStreak >= windowSec && !w.someHit {
						w.someHit = true
						watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "memory_pressure",
							fmt.Sprintf("memory pressure some=%.1f%% sustained (threshold %.0f%%)", mem.Some10, mp.SomePct),
							map[string]float64{"psi_some": mem.Some10}))
					}
				} else {
					w.someStreak = 0
					w.someHit = false
				}
				if mp.FullPct > 0 && mem.Full10 > mp.FullPct {
					w.fullStreak++
					if w.fullStreak >= windowSec && !w.fullHit {
						w.fullHit = true
						watcher.Emit(emit, watcher.NewEvent(watcher.Critical, "memory_pressure",
							fmt.Sprintf("memory pressure full=%.1f%% sustained (threshold %.0f%%)", mem.Full10, mp.FullPct),
							map[string]float64{"psi_full": mem.Full10}))
					}
				} else {
					w.fullStreak = 0
					w.fullHit = false
				}
			}
			if c, err := Read("cpu"); err == nil {
				w.cpu10 = c.Some10
			}
			if io, err := Read("io"); err == nil {
				w.io10 = io.Some10
				w.ioFull = io.Full10
			}
		}
	}
}

func (w *Watcher) Snapshot() map[string]float64 {
	return map[string]float64{
		"psi_mem_some": w.some10,
		"psi_mem_full": w.full10,
		"psi_cpu_some": w.cpu10,
		"psi_io_some":  w.io10,
		"psi_io_full":  w.ioFull,
	}
}
