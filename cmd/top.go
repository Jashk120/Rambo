package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/jashk120/rambo/internal/monitor"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show live system stats and top RAM consumers",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runTop()
	},
}

func runTop() error {
	mem, err := monitor.GetMemInfo()
	if err != nil {
		return err
	}

	// take two snapshots 500ms apart for CPU/network/disk rates
	prevNet, err := monitor.SnapshotNet()
	if err != nil {
		return err
	}
	prevIO, err := monitor.SnapshotDiskIO()
	if err != nil {
		return err
	}
	prevOverall, prevPerCore, err := monitor.ReadCPURaw()
	if err != nil {
		return err
	}
	prevCPU := struct {
		Overall monitor.CPURaw
		PerCore []monitor.CPURaw
	}{prevOverall, prevPerCore}

	time.Sleep(500 * time.Millisecond)

	netStats, _, err := monitor.GetIfaceStats(prevNet, 0.5)
	if err != nil {
		return err
	}
	ioStats, _, err := monitor.GetDiskIOStats(prevIO, 0.5)
	if err != nil {
		return err
	}
	curOverall, curPerCore, err := monitor.ReadCPURaw()
	if err != nil {
		return err
	}
	curCPU := struct {
		Overall monitor.CPURaw
		PerCore []monitor.CPURaw
	}{curOverall, curPerCore}
	cpuStats := monitor.CalcCPUStats(prevCPU, curCPU)

	spaceStats, err := monitor.GetDiskSpaceInfo()
	if err != nil {
		return err
	}

	procs, err := monitor.GetTopProcesses(10)
	if err != nil {
		return err
	}

	// RAM
	usedGB := float64(mem.Used) / 1024 / 1024
	totalGB := float64(mem.Total) / 1024 / 1024
	pct := 0.0
	if mem.Total > 0 {
		pct = float64(mem.Used) / float64(mem.Total) * 100
	}
	drawBox("RAM", []string{fmt.Sprintf("%.1f GB / %.1f GB used  (%.1f%%)", usedGB, totalGB, pct)})

	// CPU
	cpuLines := []string{fmt.Sprintf("Overall: %.1f%%", cpuStats.Overall)}
	parts := []string{}
	for i, p := range cpuStats.PerCore {
		parts = append(parts, fmt.Sprintf("Core %d: %.1f%%", i, p))
		if (i+1)%4 == 0 {
			cpuLines = append(cpuLines, strings.Join(parts, "  "))
			parts = []string{}
		}
	}
	if len(parts) > 0 {
		cpuLines = append(cpuLines, strings.Join(parts, "  "))
	}
	drawBox("CPU", cpuLines)

	// Network
	netLines := []string{}
	for _, n := range netStats {
		netLines = append(netLines, fmt.Sprintf("%-6s ↓ %6.1f MB/s  ↑ %6.1f MB/s",
			n.Name, n.RxBytesPerSec/1e6, n.TxBytesPerSec/1e6))
	}
	if len(netLines) == 0 {
		netLines = []string{"No active interfaces"}
	}
	drawBox("Network", netLines)

	// Disk I/O
	ioLines := []string{}
	for _, d := range ioStats {
		ioLines = append(ioLines, fmt.Sprintf("%-10s  R %7.1f MB/s  W %7.1f MB/s",
			d.Device, d.ReadMBPerSec, d.WriteMBPerSec))
	}
	if len(ioLines) == 0 {
		ioLines = []string{"No physical disks"}
	}
	drawBox("Disk I/O", ioLines)

	// Disk Space
	spaceLines := []string{}
	for _, d := range spaceStats {
		spaceLines = append(spaceLines, fmt.Sprintf("%-12s %8.1f / %8.1f GB  %5.1f%%",
			d.Mount, d.UsedGB, d.TotalGB, d.UsedPct))
	}
	drawBox("Disk Space", spaceLines)

	// Top RAM consumers
	procLines := []string{fmt.Sprintf("%-8s %-20s %s", "PID", "NAME", "RAM")}
	for _, p := range procs {
		procLines = append(procLines, fmt.Sprintf("%-8d %-20s %d MB", p.PID, p.Name, p.RSS/1024))
	}
	drawBox("Top RAM Consumers", procLines)

	return nil
}

// drawBox prints a titled box with the given content lines.
func drawBox(title string, lines []string) {
	inner := 0
	for _, l := range lines {
		if len(l) > inner {
			inner = len(l)
		}
	}
	if len(title)+2 > inner {
		inner = len(title) + 2
	}
	fmt.Printf("  ┌─ %s %s┐\n", title, strings.Repeat("─", inner-len(title)))
	for _, l := range lines {
		fmt.Printf("  │ %s%s │\n", l, strings.Repeat(" ", inner-len(l)))
	}
	fmt.Printf("  └%s┘\n", strings.Repeat("─", inner+2))
	fmt.Println()
}

func init() {
	rootCmd.AddCommand(topCmd)
}
