package memory

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jashk120/rambo/internal/config"
	"github.com/jashk120/rambo/internal/watcher"
)

type limitDir struct {
	dir         string
	restoreHigh string
	restoreMax  string
}

type Watcher struct {
	cfg        config.Config
	monitorDir string
	limitDirs  []limitDir
	totalKB    uint64
	softKB     uint64
	hardKB     uint64
	maxKB      uint64

	prevHigh uint64
	prevMax  uint64
	prevOOM  uint64
	baseline bool
	softHit  bool
	hardHit  bool
	maxHit   bool
	oomHit   bool

	usedKB uint64
}

// SessionRoot finds the user session cgroup under /sys/fs/cgroup
// (e.g. user.slice/user-1000.slice/user@1000.service).
func SessionRoot() (string, error) {
	data, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(string(data))
	parts := strings.SplitN(line, "::", 2)
	if len(parts) < 2 {
		return "", errors.New("unexpected /proc/self/cgroup format")
	}
	path := parts[1]
	idx := strings.Index(path, "user@")
	if idx < 0 {
		root := filepath.Join("/sys/fs/cgroup", path)
		if _, err := os.Stat(root); err != nil {
			return "", err
		}
		return root, nil
	}
	rest := path[idx:]
	segs := strings.SplitN(rest, "/", 2)
	if len(segs) == 0 {
		return "", errors.New("no session cgroup")
	}
	root := filepath.Join("/sys/fs/cgroup", path[:idx], segs[0])
	if _, err := os.Stat(root); err != nil {
		return "", err
	}
	return root, nil
}

// writableLimitDirs returns the desktop slices under the session root whose
// memory.high the user can write. Only app.slice and session.slice are
// managed — they cover desktop apps and Plasma services. init.scope and
// background.slice are deliberately excluded.
func writableLimitDirs(root string) []string {
	var out []string
	for _, name := range []string{"app.slice", "session.slice"} {
		dir := filepath.Join(root, name)
		if syscall.Access(filepath.Join(dir, "memory.high"), 0o2) == nil {
			out = append(out, dir)
		}
	}
	return out
}

func NewWatcher(cfg config.Config) (*Watcher, error) {
	total := config.TotalRAMKB()
	if total == 0 {
		return nil, errors.New("cannot determine total RAM")
	}
	w := &Watcher{
		cfg:     cfg,
		totalKB: total,
		softKB:  uint64(float64(total) * cfg.Memory.SoftPct / 100),
		hardKB:  uint64(float64(total) * cfg.Memory.HardPct / 100),
		maxKB:   uint64(float64(total) * cfg.Memory.MaxPct / 100),
	}
	root, err := SessionRoot()
	if err != nil {
		w.monitorDir = ""
		return w, nil // degrade to meminfo-based monitoring
	}
	w.monitorDir = root
	for _, dir := range writableLimitDirs(root) {
		w.applyLimits(dir)
	}
	return w, nil
}

func (w *Watcher) Name() string { return "memory" }

// LimitDirs returns the cgroups the kernel-enforced limits were applied to.
func (w *Watcher) LimitDirs() []string {
	out := make([]string, 0, len(w.limitDirs))
	for _, l := range w.limitDirs {
		out = append(out, l.dir)
	}
	return out
}

func (w *Watcher) MonitorDir() string { return w.monitorDir }

func (w *Watcher) applyLimits(dir string) {
	i := -1
	for k, l := range w.limitDirs {
		if l.dir == dir {
			i = k
			break
		}
	}
	if i < 0 {
		w.limitDirs = append(w.limitDirs, limitDir{dir: dir})
		i = len(w.limitDirs) - 1
	}
	if v, err := readFile(filepath.Join(dir, "memory.high")); err == nil {
		w.limitDirs[i].restoreHigh = strings.TrimSpace(v)
	}
	if v, err := readFile(filepath.Join(dir, "memory.max")); err == nil {
		w.limitDirs[i].restoreMax = strings.TrimSpace(v)
	}
	// cgroup v2 memory.high/memory.max are in bytes. Guard against any future
	// unit error ever writing a sub-GiB limit (which would OOM-kill the cgroup).
	const minHigh, minMax = 512 << 20, 1 << 30
	high := w.softKB * 1024
	maxv := w.maxKB * 1024
	if high < minHigh || maxv < minMax {
		fmt.Printf("[rambo] REFUSING to set limits on %s: high=%dB max=%dB below safety floor\n", dir, high, maxv)
		return
	}
	if err := writeFile(filepath.Join(dir, "memory.high"), strconv.FormatUint(high, 10)); err != nil {
		fmt.Printf("[rambo] cannot set memory.high on %s: %v\n", dir, err)
	}
	if err := writeFile(filepath.Join(dir, "memory.max"), strconv.FormatUint(maxv, 10)); err != nil {
		fmt.Printf("[rambo] cannot set memory.max on %s: %v\n", dir, err)
	}
}

func (w *Watcher) Restore() {
	for _, l := range w.limitDirs {
		high := l.restoreHigh
		if high == "" {
			high = "max"
		}
		maxv := l.restoreMax
		if maxv == "" {
			maxv = "max"
		}
		_ = writeFile(filepath.Join(l.dir, "memory.high"), high)
		_ = writeFile(filepath.Join(l.dir, "memory.max"), maxv)
	}
}

func (w *Watcher) Run(ctx context.Context, emit chan<- watcher.Event) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			w.tick(emit)
		}
	}
}

func (w *Watcher) tick(emit chan<- watcher.Event) {
	used := w.currentKB()
	w.usedKB = used

	if len(w.limitDirs) > 0 {
		w.readEventsDiff(emit)
	}

	if used >= w.hardKB && !w.hardHit {
		w.hardHit = true
		w.softHit = true
		watcher.Emit(emit, watcher.NewEvent(watcher.Critical, "memory",
			fmt.Sprintf("RAM at %.1f GB / %.1f GB (%.0f%%) — killing top consumer",
				float64(used)/1048576, float64(w.totalKB)/1048576,
				float64(used)/float64(w.totalKB)*100), nil))
	} else if used >= w.softKB && !w.softHit {
		w.softHit = true
		watcher.Emit(emit, watcher.NewEvent(watcher.Warning, "memory",
			fmt.Sprintf("RAM at %.1f GB / %.1f GB (%.0f%%) — approaching limit",
				float64(used)/1048576, float64(w.totalKB)/1048576,
				float64(used)/float64(w.totalKB)*100), nil))
	} else if used < w.softKB {
		w.softHit = false
		w.hardHit = false
	}
}

func (w *Watcher) readEventsDiff(emit chan<- watcher.Event) {
	var high, maxv, oom uint64
	for _, l := range w.limitDirs {
		data, err := readFile(filepath.Join(l.dir, "memory.events"))
		if err != nil {
			continue
		}
		ev := parseEvents(data)
		high += ev["high"]
		maxv += ev["max"]
		oom += ev["oom_kill"]
	}
	if !w.baseline {
		w.prevHigh = high
		w.prevMax = maxv
		w.prevOOM = oom
		w.baseline = true
		return
	}
	if maxv > w.prevMax {
		w.prevMax = maxv
		if !w.maxHit {
			w.maxHit = true
			watcher.Emit(emit, watcher.NewEvent(watcher.Emergency, "memory",
				"kernel hit memory.max in your session", nil))
		}
	} else if maxv == w.prevMax {
		w.maxHit = false
	}
	if oom > w.prevOOM {
		w.prevOOM = oom
		if !w.oomHit {
			w.oomHit = true
			watcher.Emit(emit, watcher.NewEvent(watcher.Emergency, "memory",
				"kernel OOM-killed a process in your session", nil))
		}
	} else if oom == w.prevOOM {
		w.oomHit = false
	}
	if high > w.prevHigh {
		w.prevHigh = high
	}
}

func (w *Watcher) currentKB() uint64 {
	if w.monitorDir != "" {
		if v, err := readFile(filepath.Join(w.monitorDir, "memory.current")); err == nil {
			if kb, perr := strconv.ParseUint(strings.TrimSpace(v), 10, 64); perr == nil {
				return kb / 1024
			}
		}
	}
	if used, err := meminfoUsedKB(); err == nil {
		return used
	}
	return 0
}

func (w *Watcher) Snapshot() map[string]float64 {
	pct := 0.0
	if w.totalKB > 0 {
		pct = float64(w.usedKB) / float64(w.totalKB) * 100
	}
	return map[string]float64{
		"mem_used_gb":  float64(w.usedKB) / 1048576,
		"mem_total_gb": float64(w.totalKB) / 1048576,
		"mem_pct":      pct,
	}
}

func parseEvents(data string) map[string]uint64 {
	out := map[string]uint64{}
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		out[fields[0]] = v
	}
	return out
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func writeFile(path, val string) error {
	return os.WriteFile(path, []byte(val), 0o644)
}

func meminfoUsedKB() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer f.Close()
	var total, available uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			total = v
		case "MemAvailable:":
			available = v
		}
	}
	if total == 0 {
		return 0, errors.New("bad meminfo")
	}
	return total - available, nil
}
