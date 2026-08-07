package monitor

import (
	"bufio"
	"os"
	"sort"
	"strconv"
	"strings"
)

// IfaceStats holds per-interface network throughput in bytes per second.
type IfaceStats struct {
	Name          string
	RxBytesPerSec float64
	TxBytesPerSec float64
}

// SnapshotNet reads /proc/net/dev and returns the raw [rxBytes, txBytes]
// counters for every non-loopback interface.
func SnapshotNet() (map[string][2]uint64, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	snap := make(map[string][2]uint64)
	scanner := bufio.NewScanner(f)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		// skip the two header lines
		if lineNum <= 2 {
			continue
		}
		line := scanner.Text()
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[:idx])
		if name == "lo" {
			continue
		}
		fields := strings.Fields(line[idx+1:])
		if len(fields) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(fields[0], 10, 64)
		tx, _ := strconv.ParseUint(fields[8], 10, 64)
		snap[name] = [2]uint64{rx, tx}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return snap, nil
}

// GetIfaceStats computes per-interface throughput (bytes/sec) by comparing the
// current snapshot against prev. On the first call pass prev == nil to get
// zeroed rates plus the raw snapshot. Results are sorted by total throughput
// descending.
func GetIfaceStats(prev map[string][2]uint64, intervalSec float64) ([]IfaceStats, map[string][2]uint64, error) {
	cur, err := SnapshotNet()
	if err != nil {
		return nil, nil, err
	}
	if prev == nil {
		stats := make([]IfaceStats, 0, len(cur))
		for name := range cur {
			stats = append(stats, IfaceStats{Name: name})
		}
		sortIface(stats)
		return stats, cur, nil
	}

	stats := make([]IfaceStats, 0, len(cur))
	for name, c := range cur {
		p, ok := prev[name]
		if !ok || c[0] < p[0] || c[1] < p[1] {
			continue // new interface or counter reset
		}
		stats = append(stats, IfaceStats{
			Name:          name,
			RxBytesPerSec: float64(c[0]-p[0]) / intervalSec,
			TxBytesPerSec: float64(c[1]-p[1]) / intervalSec,
		})
	}
	sortIface(stats)
	return stats, cur, nil
}

func sortIface(stats []IfaceStats) {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].RxBytesPerSec+stats[i].TxBytesPerSec > stats[j].RxBytesPerSec+stats[j].TxBytesPerSec
	})
}
