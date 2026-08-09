package disk

import (
	"context"
	"fmt"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/monitor"
	"github.com/jashk120/rambo/internal/watcher"
)

type Watcher struct {
	cfg         config.Config
	maxSpacePct float64
	topIO       float64
	spaceStreak int
	spaceHit    bool
	ioStreak    int
	ioHit       bool
}

func NewWatcher(cfg config.Config) *Watcher {
	return &Watcher{cfg: cfg}
}

func (w *Watcher) Name() string { return "disk" }

func (w *Watcher) Snapshot() map[string]float64 {
	return map[string]float64{"disk_max_pct": w.maxSpacePct, "io_top_mbps": w.topIO}
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	prevIO, err := monitor.SnapshotDiskIO()
	if err != nil {
		prevIO = nil
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.checkSpace(emit)
			w.checkIO(emit, &prevIO)
		}
	}
}

func (w *Watcher) checkSpace(emit chan<- watcher.Event) {
	infos, err := monitor.GetDiskSpaceInfo()
	if err != nil {
		return
	}
	maxPct := 0.0
	where := ""
	for _, d := range infos {
		if d.UsedPct > maxPct {
			maxPct = d.UsedPct
			where = d.Mount
		}
	}
	w.maxSpacePct = maxPct
	thresh := w.cfg.Disk.SpaceAlertPct
	if thresh <= 0 {
		return
	}
	if maxPct > thresh {
		w.spaceStreak++
		if w.spaceStreak >= 2 && !w.spaceHit {
			w.spaceHit = true
			watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "disk",
				fmt.Sprintf("%s at %.1f%% used (threshold %.0f%%)", where, maxPct, thresh),
				map[string]float64{"disk_pct": maxPct}))
		}
	} else {
		w.spaceStreak = 0
	}
	if maxPct < thresh-3 {
		w.spaceHit = false
	}
}

func (w *Watcher) checkIO(emit chan<- watcher.Event, prev *map[string][2]uint64) {
	if !w.cfg.Disk.IOAlert {
		return
	}
	stats, cur, err := monitor.GetDiskIOStats(*prev, 10)
	if err != nil {
		return
	}
	*prev = cur
	top := 0.0
	dev := ""
	for _, s := range stats {
		if t := s.ReadMBPerSec + s.WriteMBPerSec; t > top {
			top = t
			dev = s.Device
		}
	}
	w.topIO = top
	if top > 1000 { // MB/s, pathological sustained IO
		w.ioStreak++
		if w.ioStreak >= 2 && !w.ioHit {
			w.ioHit = true
			watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "disk",
				fmt.Sprintf("%s at %.0f MB/s total IO", dev, top),
				map[string]float64{"io_mbps": top}))
		}
	} else {
		w.ioStreak = 0
	}
	if top < 500 {
		w.ioHit = false
	}
}
