package cmd

import (
	"context"
	"fmt"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jashk120/rambo/internal/monitor"
	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Live view of system stats (Ctrl+C to exit)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		intervalSec := 2.0

		prevNet, _ := monitor.SnapshotNet()
		prevIO, _ := monitor.SnapshotDiskIO()
		var prevCPU struct {
			Overall monitor.CPURaw
			PerCore []monitor.CPURaw
		}
		if o, c, err := monitor.ReadCPURaw(); err == nil {
			prevCPU.Overall = o
			prevCPU.PerCore = c
		}

		for {
			select {
			case <-ctx.Done():
				fmt.Print("\033[2J\033[H")
				return nil
			default:
			}

			mem, _ := monitor.GetMemInfo()
			netStats, newNet, _ := monitor.GetIfaceStats(prevNet, intervalSec)
			prevNet = newNet
			ioStats, newIO, _ := monitor.GetDiskIOStats(prevIO, intervalSec)
			prevIO = newIO
			curOverall, curPerCore, _ := monitor.ReadCPURaw()
			curCPU := struct {
				Overall monitor.CPURaw
				PerCore []monitor.CPURaw
			}{curOverall, curPerCore}
			cpuStats := monitor.CalcCPUStats(prevCPU, curCPU)
			prevCPU = curCPU
			spaceStats, _ := monitor.GetDiskSpaceInfo()

			// RAM
			usedGB := float64(mem.Used) / 1024 / 1024
			totalGB := float64(mem.Total) / 1024 / 1024
			pct := 0.0
			if mem.Total > 0 {
				pct = float64(mem.Used) / float64(mem.Total) * 100
			}
			ramLines := []string{fmt.Sprintf("%.1f GB / %.1f GB used  (%.1f%%)", usedGB, totalGB, pct)}

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

			// Network
			netLines := []string{}
			for _, n := range netStats {
				netLines = append(netLines, fmt.Sprintf("%-6s ↓ %6.1f MB/s  ↑ %6.1f MB/s",
					n.Name, n.RxBytesPerSec/1e6, n.TxBytesPerSec/1e6))
			}
			if len(netLines) == 0 {
				netLines = []string{"No active interfaces"}
			}

			// Disk I/O
			ioLines := []string{}
			for _, d := range ioStats {
				ioLines = append(ioLines, fmt.Sprintf("%-10s  R %7.1f MB/s  W %7.1f MB/s",
					d.Device, d.ReadMBPerSec, d.WriteMBPerSec))
			}
			if len(ioLines) == 0 {
				ioLines = []string{"No physical disks"}
			}

			// Disk Space
			spaceLines := []string{}
			for _, d := range spaceStats {
				spaceLines = append(spaceLines, fmt.Sprintf("%-12s %8.1f / %8.1f GB  %5.1f%%",
					d.Mount, d.UsedGB, d.TotalGB, d.UsedPct))
			}

			fmt.Print("\033[2J\033[H")
			fmt.Println("rambo stats — Ctrl+C to exit")
			fmt.Println()
			drawBox("RAM", ramLines)
			drawBox("CPU", cpuLines)
			drawBox("Network", netLines)
			drawBox("Disk I/O", ioLines)
			drawBox("Disk Space", spaceLines)

			time.Sleep(time.Duration(intervalSec) * time.Second)
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
