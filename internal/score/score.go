package score

import (
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jashk120/rambo/internal/config"
)

const hz = 100

type Candidate struct {
	PID         int
	Name        string
	RSS         uint64
	CPU         float64
	ElapsedSec  uint64
	Interactive bool
}

type sample struct {
	pid, tty int
	name     string
	rss      uint64
	start    uint64
	utime    uint64
	stime    uint64
}

// Collect walks /proc twice (500ms apart) to compute per-process CPU and
// returns kill candidates. The /proc walk only runs on demand (kill events).
func Collect(exclude map[string]bool) ([]Candidate, error) {
	first, err := readAll()
	if err != nil {
		return nil, err
	}
	time.Sleep(500 * time.Millisecond)
	second, err := readAll()
	if err != nil {
		return nil, err
	}
	nowTicks := uint64(uptimeSec() * hz)

	var out []Candidate
	for pid, s := range second {
		f, ok := first[pid]
		if !ok {
			continue
		}
		if s.name == "rambo" {
			continue
		}
		if exclude[s.name] {
			continue
		}
		dt := int64(s.utime+s.stime) - int64(f.utime+f.stime)
		if dt < 0 {
			dt = 0
		}
		cpu := float64(dt) / (0.5 * float64(hz)) * 100
		if cpu > 100 {
			cpu = 100
		}
		elapsed := uint64(0)
		if nowTicks > s.start {
			elapsed = (nowTicks - s.start) / hz
		}
		out = append(out, Candidate{
			PID:         pid,
			Name:        s.name,
			RSS:         s.rss,
			CPU:         cpu,
			ElapsedSec:  elapsed,
			Interactive: s.tty != 0,
		})
	}
	return out, nil
}

// Compute returns a kill score in [0, +inf). Higher = better kill target.
// RSS and CPU dominate; younger processes score higher; interactive processes
// (controlling TTY) and expendable ones get penalties.
func Compute(c Candidate, w config.KillWeights, totalRAM uint64, expendable bool) float64 {
	rssNorm := clamp01(float64(c.RSS) / float64(totalRAM))
	cpuNorm := clamp01(c.CPU / 100)
	rt := clamp01(1 - float64(c.ElapsedSec)/3600/4)
	s := w.RSS*rssNorm + w.CPU*cpuNorm + w.Runtime*rt
	if c.Interactive {
		s += 0.3
	}
	if expendable {
		s += 0.2
	}
	return s
}

func Rank(cands []Candidate, w config.KillWeights, totalRAM uint64, expendable map[string]bool) []Candidate {
	sorted := make([]Candidate, len(cands))
	copy(sorted, cands)
	sort.Slice(sorted, func(i, j int) bool {
		return Compute(sorted[i], w, totalRAM, expendable[sorted[i].Name]) >
			Compute(sorted[j], w, totalRAM, expendable[sorted[j].Name])
	})
	return sorted
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func readAll() (map[int]*sample, error) {
	entries, err := filepath.Glob("/proc/[0-9]*/stat")
	if err != nil {
		return nil, err
	}
	out := map[int]*sample{}
	for _, path := range entries {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := parseStat(string(b))
		if s == nil {
			continue
		}
		s.rss = readRSS(filepath.Dir(path) + "/status")
		out[s.pid] = s
	}
	return out, nil
}

func parseStat(s string) *sample {
	open := strings.Index(s, "(")
	close := strings.LastIndex(s, ")")
	if open < 0 || close < 0 || close+1 >= len(s) {
		return nil
	}
	pid, err := strconv.Atoi(strings.TrimSpace(s[:open]))
	if err != nil {
		return nil
	}
	name := s[open+1 : close]
	toks := strings.Fields(s[close+1:])
	if len(toks) < 20 {
		return nil
	}
	utime, _ := strconv.ParseUint(toks[11], 10, 64)
	stime, _ := strconv.ParseUint(toks[12], 10, 64)
	start, _ := strconv.ParseUint(toks[19], 10, 64)
	tty, _ := strconv.Atoi(toks[4])
	return &sample{
		pid:   pid,
		name:  name,
		tty:   tty,
		start: start,
		utime: utime,
		stime: stime,
	}
}

func readRSS(statusPath string) uint64 {
	b, err := os.ReadFile(statusPath)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "VmRSS:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				v, _ := strconv.ParseUint(fields[1], 10, 64)
				return v * 1024
			}
		}
	}
	return 0
}

func uptimeSec() float64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(b))
	if len(fields) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(fields[0], 64)
	return v
}
