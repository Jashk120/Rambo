package cpu

import (
	"context"
	"fmt"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/monitor"
	"github.com/jashk120/rambo/internal/watcher"
)

type Watcher struct {
	cfg     config.Config
	overall float64
	streak  int
	alerted bool
}

func NewWatcher(cfg config.Config) *Watcher {
	return &Watcher{cfg: cfg}
}

func (w *Watcher) Name() string { return "cpu" }

func (w *Watcher) Snapshot() map[string]float64 {
	return map[string]float64{"cpu_pct": w.overall}
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	prevO, prevC, err := monitor.ReadCPURaw()
	if err != nil {
		return err
	}
	prev := struct {
		Overall monitor.CPURaw
		PerCore []monitor.CPURaw
	}{prevO, prevC}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			curO, curC, err := monitor.ReadCPURaw()
			if err != nil {
				continue
			}
			cur := struct {
				Overall monitor.CPURaw
				PerCore []monitor.CPURaw
			}{curO, curC}
			st := monitor.CalcCPUStats(prev, cur)
			prev = cur
			w.overall = st.Overall

			thresh := w.cfg.CPU.AlertPct
			if thresh <= 0 {
				continue
			}
			if st.Overall > thresh {
				w.streak++
				if w.streak >= 3 && !w.alerted {
					w.alerted = true
					watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "cpu",
						fmt.Sprintf("CPU at %.1f%% (threshold %.0f%%)", st.Overall, thresh),
						map[string]float64{"cpu_pct": st.Overall}))
				}
			} else {
				w.streak = 0
			}
			if st.Overall < thresh-5 {
				w.alerted = false
			}
		}
	}
}
