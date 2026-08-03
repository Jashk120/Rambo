package monitor

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type MemInfo struct {
	Total     uint64
	Available uint64
	Used      uint64
}

type Process struct {
	PID  int
	Name string
	RSS  uint64
}

func GetMemInfo() (MemInfo, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return MemInfo{}, err
	}
	defer f.Close()

	info := MemInfo{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, _ := strconv.ParseUint(fields[1], 10, 64)
		switch fields[0] {
		case "MemTotal:":
			info.Total = val
		case "MemAvailable:":
			info.Available = val
		}
	}
	info.Used = info.Total - info.Available
	return info, nil
}

func GetTopProcesses(n int) ([]Process, error) {
	procs := []Process{}

	entries, err := filepath.Glob("/proc/[0-9]*/status")
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		f, err := os.Open(entry)
		if err != nil {
			continue
		}

		var name string
		var rss uint64
		var pid int

		fmt.Sscanf(entry, "/proc/%d/status", &pid)

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			switch fields[0] {
			case "Name:":
				name = fields[1]
			case "VmRSS:":
				rss, _ = strconv.ParseUint(fields[1], 10, 64)
			}
		}
		f.Close()

		if rss > 0 {
			procs = append(procs, Process{PID: pid, Name: name, RSS: rss})
		}
	}

	sort.Slice(procs, func(i, j int) bool {
		return procs[i].RSS > procs[j].RSS
	})

	if len(procs) > n {
		return procs[:n], nil
	}
	return procs, nil
}

type ThresholdAction func(used, total uint64, hard bool)

func Watch(softGB, hardGB float64, intervalSec int, action ThresholdAction) {
	softKB := uint64(softGB * 1024 * 1024)
	hardKB := uint64(hardGB * 1024 * 1024)

	softTriggered := false
	hardTriggered := false

	for {
		mem, err := GetMemInfo()
		if err != nil {
			time.Sleep(time.Duration(intervalSec) * time.Second)
			continue
		}

		if mem.Used >= hardKB && !hardTriggered {
			hardTriggered = true
			softTriggered = true
			action(mem.Used, mem.Total, true)
		} else if mem.Used >= softKB && !softTriggered {
			softTriggered = true
			action(mem.Used, mem.Total, false)
		} else if mem.Used < softKB {
			// reset both once RAM drops back down
			softTriggered = false
			hardTriggered = false
		}

		time.Sleep(time.Duration(intervalSec) * time.Second)
	}
}
