package cmd

import (
	"fmt"

	"github.com/jashk120/rambo/internal/killer"
	"github.com/jashk120/rambo/internal/monitor"
	"github.com/jashk120/rambo/internal/notify"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"time"
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

		monitor.Watch(cfg.SoftGB, cfg.HardGB, 5, func(used, total uint64, hard bool) {
			usedMB := used / 1024
			totalMB := total / 1024

			if hard {
				notify.Send(
					"⚠️ RAMBO: Hard threshold hit",
					fmt.Sprintf("RAM at %d/%d MB — killing top consumer", usedMB, totalMB),
				)
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
			} else {
				notify.Send(
					"⚡ RAMBO: Soft threshold hit",
					fmt.Sprintf("RAM at %d/%d MB — approaching limit", usedMB, totalMB),
				)
			}
		})

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
