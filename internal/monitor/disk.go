package monitor

import (
	"bufio"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
)

// DiskIOStats holds per-device throughput in MB/s.
type DiskIOStats struct {
	Device        string
	ReadMBPerSec  float64
	WriteMBPerSec float64
}

// DiskSpaceInfo describes the usage of a mounted filesystem.
type DiskSpaceInfo struct {
	Mount   string
	UsedGB  float64
	TotalGB float64
	UsedPct float64
}

// physical disks only: sda/sdb..., nvme0n1..., vda/vdb... (no partitions).
var physDiskRE = regexp.MustCompile(`^(sd[a-z]|nvme[0-9]+n[0-9]+|vd[a-z])$`)

// SnapshotDiskIO reads /proc/diskstats and returns the raw
// [sectorsRead, sectorsWritten] counters for each physical disk.
func SnapshotDiskIO() (map[string][2]uint64, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	snap := make(map[string][2]uint64)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}
		name := fields[2]
		if !physDiskRE.MatchString(name) {
			continue
		}
		read, _ := strconv.ParseUint(fields[5], 10, 64)
		written, _ := strconv.ParseUint(fields[9], 10, 64)
		snap[name] = [2]uint64{read, written}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return snap, nil
}

// GetDiskIOStats computes per-device throughput (MB/s) by comparing the current
// snapshot against prev. On the first call pass prev == nil to get zeroed rates
// plus the raw snapshot. Results are sorted by total throughput descending.
func GetDiskIOStats(prev map[string][2]uint64, intervalSec float64) ([]DiskIOStats, map[string][2]uint64, error) {
	cur, err := SnapshotDiskIO()
	if err != nil {
		return nil, nil, err
	}
	if prev == nil {
		stats := make([]DiskIOStats, 0, len(cur))
		for name := range cur {
			stats = append(stats, DiskIOStats{Device: name})
		}
		sortDiskIO(stats)
		return stats, cur, nil
	}

	stats := make([]DiskIOStats, 0, len(cur))
	for name, c := range cur {
		p, ok := prev[name]
		if !ok || c[0] < p[0] || c[1] < p[1] {
			continue // new device or counter reset
		}
		stats = append(stats, DiskIOStats{
			Device:        name,
			ReadMBPerSec:  float64(c[0]-p[0]) * 512 / intervalSec / 1024 / 1024,
			WriteMBPerSec: float64(c[1]-p[1]) * 512 / intervalSec / 1024 / 1024,
		})
	}
	sortDiskIO(stats)
	return stats, cur, nil
}

func sortDiskIO(stats []DiskIOStats) {
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].ReadMBPerSec+stats[i].WriteMBPerSec > stats[j].ReadMBPerSec+stats[j].WriteMBPerSec
	})
}

// GetDiskSpaceInfo reports used/total GB and usage percent for every real
// mounted filesystem, sorted by UsedPct descending.
func GetDiskSpaceInfo() ([]DiskSpaceInfo, error) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	skip := map[string]bool{
		"tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
		"cgroup": true, "cgroup2": true, "devpts": true, "mqueue": true,
		"hugetlbfs": true, "debugfs": true, "tracefs": true, "pstore": true,
		"bpf": true, "fusectl": true, "efivarfs": true, "securityfs": true,
		"configfs": true, "binfmt_misc": true, "autofs": true, "nsfs": true,
		"ramfs": true,
	}

	infos := []DiskSpaceInfo{}
	seen := make(map[string]bool)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}
		if skip[fields[2]] || strings.HasPrefix(fields[2], "fuse.") {
			continue
		}
		mount := fields[1]
		if seen[mount] {
			continue
		}
		seen[mount] = true
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount, &stat); err != nil {
			continue
		}
		total := stat.Blocks * uint64(stat.Bsize)
		free := stat.Bavail * uint64(stat.Bsize)
		used := total - free
		pct := 0.0
		if total > 0 {
			pct = float64(used) / float64(total) * 100
		}
		infos = append(infos, DiskSpaceInfo{
			Mount:   mount,
			UsedGB:  float64(used) / 1024 / 1024 / 1024,
			TotalGB: float64(total) / 1024 / 1024 / 1024,
			UsedPct: pct,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].UsedPct > infos[j].UsedPct
	})
	return infos, nil
}
