package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jashk120/rambo/internal/killer"
	"github.com/jashk120/rambo/internal/monitor"
	"github.com/jashk120/rambo/internal/notify"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start the rambo background daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("no config found, run 'rambo threshold set' first")
		}

		fmt.Printf("[rambo] daemon started — soft: %.1f GB, hard: %.1f GB\n", cfg.SoftGB, cfg.HardGB)

		softKB := uint64(cfg.SoftGB * 1024 * 1024)
		hardKB := uint64(cfg.HardGB * 1024 * 1024)
		softTriggered := false
		hardTriggered := false

		intervalSec := 5.0
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()

		// initialise prev snapshots before the loop
		prevNet, err := monitor.SnapshotNet()
		if err != nil {
			prevNet = nil
		}
		prevIO, err := monitor.SnapshotDiskIO()
		if err != nil {
			prevIO = nil
		}
		var prevCPU struct {
			Overall monitor.CPURaw
			PerCore []monitor.CPURaw
		}
		if o, c, err := monitor.ReadCPURaw(); err == nil {
			prevCPU.Overall = o
			prevCPU.PerCore = c
		}

		cpuPct := 0.0

		for range ticker.C {
			// RAM
			mem, memErr := monitor.GetMemInfo()
			if memErr == nil {
				if mem.Used >= hardKB && !hardTriggered {
					hardTriggered = true
					softTriggered = true
					usedMB := mem.Used / 1024
					totalMB := mem.Total / 1024

					notify.Send(
						"⚠️ RAMBO: Hard threshold hit",
						fmt.Sprintf("RAM at %d/%d MB — killing top consumer", usedMB, totalMB),
					)
					killer.LoadWhitelist(cfg.Whitelist)
					name, pid, err := killer.KillTopConsumer()
					if err != nil {
						notify.Send("RAMBO: Kill failed", err.Error())
					} else {
						notify.Send(
							"RAMBO: Process killed",
							fmt.Sprintf("Killed %s (PID %d)", name, pid),
						)
						logKill(name, pid)
					}
				} else if mem.Used >= softKB && !softTriggered {
					softTriggered = true
					usedMB := mem.Used / 1024
					totalMB := mem.Total / 1024
					notify.Send(
						"⚡ RAMBO: Soft threshold hit",
						fmt.Sprintf("RAM at %d/%d MB — approaching limit", usedMB, totalMB),
					)
				} else if mem.Used < softKB {
					softTriggered = false
					hardTriggered = false
				}
			}

			// Network
			netStats, newNet, _ := monitor.GetIfaceStats(prevNet, intervalSec)
			prevNet = newNet
			if cfg.NetAlertMbps > 0 {
				for _, iface := range netStats {
					totalMBps := (iface.RxBytesPerSec + iface.TxBytesPerSec) / 1e6
					if totalMBps > cfg.NetAlertMbps {
						notify.Send(
							"⚡ RAMBO: High network",
							fmt.Sprintf("%s: %.1f MB/s", iface.Name, totalMBps),
						)
					}
				}
			}

			// CPU
			curOverall, curPerCore, err := monitor.ReadCPURaw()
			if err == nil {
				curCPU := struct {
					Overall monitor.CPURaw
					PerCore []monitor.CPURaw
				}{curOverall, curPerCore}
				cpuStats := monitor.CalcCPUStats(prevCPU, curCPU)
				prevCPU = curCPU
				cpuPct = cpuStats.Overall
				if cfg.CPUAlertPct > 0 && cpuStats.Overall > cfg.CPUAlertPct {
					notify.Send("⚡ RAMBO: High CPU", fmt.Sprintf("CPU at %.1f%%", cpuStats.Overall))
				}
			}

			// Disk I/O (log only, no kill action)
			ioStats, newIO, _ := monitor.GetDiskIOStats(prevIO, intervalSec)
			prevIO = newIO

			// Disk Space
			spaceStats, _ := monitor.GetDiskSpaceInfo()
			if cfg.DiskAlertPct > 0 {
				for _, d := range spaceStats {
					if d.UsedPct > cfg.DiskAlertPct {
						notify.Send(
							"⚡ RAMBO: Disk nearly full",
							fmt.Sprintf("%s at %.1f%%", d.Mount, d.UsedPct),
						)
					}
				}
			}

			// periodic status line to stdout (shows up in journalctl)
			usedMB, totalMB := uint64(0), uint64(0)
			if memErr == nil {
				usedMB = mem.Used / 1024
				totalMB = mem.Total / 1024
			}
			netLine := "-"
			if len(netStats) > 0 {
				top := netStats[0]
				netLine = fmt.Sprintf("%s ↓%.1fMB/s ↑%.1fMB/s",
					top.Name, top.RxBytesPerSec/1e6, top.TxBytesPerSec/1e6)
			}
			ioLine := "-"
			if len(ioStats) > 0 {
				top := ioStats[0]
				ioLine = fmt.Sprintf("%s R%.1f W%.1fMB/s", top.Device, top.ReadMBPerSec, top.WriteMBPerSec)
			}
			fmt.Printf("[rambo] RAM %dMB/%dMB | CPU %.1f%% | net %s | io %s\n", usedMB, totalMB, cpuPct, netLine, ioLine)
		}

		return nil
	},
}

func logKill(name string, pid int) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".local", "share", "rambo", "rambo.log")
	os.MkdirAll(filepath.Dir(path), 0755)

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "[%s] killed %s (PID %d)\n",
		time.Now().Format("2006-01-02 15:04:05"), name, pid)
}
func init() {
	rootCmd.AddCommand(daemonCmd)
}
