package monitor

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// CPUStats holds CPU usage percentages (0.0-100.0).
type CPUStats struct {
	Overall float64   // whole-system usage percent
	PerCore []float64 // one entry per logical CPU
}

// CPURaw holds the raw tick counters from /proc/stat for one CPU line.
type CPURaw struct {
	Total uint64 // total ticks across all accounted states
	Used  uint64 // Total - idle - iowait
}

// ReadCPURaw parses /proc/stat and returns the overall and per-core snapshots.
func ReadCPURaw() (overall CPURaw, perCore []CPURaw, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return CPURaw{}, nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu") {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		var total uint64
		for i := 1; i <= 8; i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
		}
		idle, _ := strconv.ParseUint(fields[4], 10, 64)
		iowait, _ := strconv.ParseUint(fields[5], 10, 64)
		raw := CPURaw{Total: total, Used: total - idle - iowait}

		if fields[0] == "cpu" {
			overall = raw
		} else {
			perCore = append(perCore, raw)
		}
	}
	if err := scanner.Err(); err != nil {
		return CPURaw{}, nil, err
	}
	return overall, perCore, nil
}

// CalcCPUStats turns two raw snapshots (read before and after a sleep interval)
// into usage percentages.
func CalcCPUStats(prev, cur struct {
	Overall CPURaw
	PerCore []CPURaw
}) CPUStats {
	stats := CPUStats{}

	totalDelta := cur.Overall.Total - prev.Overall.Total
	usedDelta := cur.Overall.Used - prev.Overall.Used
	if totalDelta > 0 {
		stats.Overall = float64(usedDelta) / float64(totalDelta) * 100
	}

	n := len(prev.PerCore)
	if len(cur.PerCore) < n {
		n = len(cur.PerCore)
	}
	stats.PerCore = make([]float64, 0, n)
	for i := 0; i < n; i++ {
		td := cur.PerCore[i].Total - prev.PerCore[i].Total
		ud := cur.PerCore[i].Used - prev.PerCore[i].Used
		if td > 0 {
			stats.PerCore = append(stats.PerCore, float64(ud)/float64(td)*100)
		} else {
			stats.PerCore = append(stats.PerCore, 0)
		}
	}
	return stats
}
