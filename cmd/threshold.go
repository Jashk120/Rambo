package cmd

import (
	"fmt"

	"github.com/jashk120/rambo/internal/config"
	"github.com/spf13/cobra"
)

var thresholdSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set memory thresholds and alert levels",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		total := config.TotalRAMKB()
		totalGB := float64(total) / 1048576

		softPct, _ := cmd.Flags().GetFloat64("soft-pct")
		hardPct, _ := cmd.Flags().GetFloat64("hard-pct")
		maxPct, _ := cmd.Flags().GetFloat64("max-pct")

		if cmd.Flags().Changed("soft") {
			g, _ := cmd.Flags().GetFloat64("soft")
			softPct = g / totalGB * 100
		}
		if cmd.Flags().Changed("hard") {
			g, _ := cmd.Flags().GetFloat64("hard")
			hardPct = g / totalGB * 100
		}
		if cmd.Flags().Changed("max") {
			g, _ := cmd.Flags().GetFloat64("max")
			maxPct = g / totalGB * 100
		}

		if softPct >= hardPct {
			return fmt.Errorf("soft threshold must be less than hard threshold")
		}
		cfg.Memory.SoftPct = softPct
		cfg.Memory.HardPct = hardPct
		if maxPct > 0 {
			if hardPct >= maxPct {
				return fmt.Errorf("max threshold must be greater than hard threshold")
			}
			cfg.Memory.MaxPct = maxPct
		}

		if cmd.Flags().Changed("net-alert") {
			cfg.Network.AlertMBps, _ = cmd.Flags().GetFloat64("net-alert")
		}
		if cmd.Flags().Changed("cpu-alert") {
			cfg.CPU.AlertPct, _ = cmd.Flags().GetFloat64("cpu-alert")
		}
		if cmd.Flags().Changed("disk-alert") {
			cfg.Disk.SpaceAlertPct, _ = cmd.Flags().GetFloat64("disk-alert")
		}
		if cmd.Flags().Changed("temp-kill") {
			cfg.Temperature.Critical, _ = cmd.Flags().GetFloat64("temp-kill")
		}

		if err := config.Save(config.Path(), cfg); err != nil {
			return err
		}
		fmt.Printf("Thresholds set — soft %.0f%% (%.1fG), hard %.0f%% (%.1fG), max %.0f%% (%.1fG)\n",
			cfg.Memory.SoftPct, gbpct(total, cfg.Memory.SoftPct),
			cfg.Memory.HardPct, gbpct(total, cfg.Memory.HardPct),
			cfg.Memory.MaxPct, gbpct(total, cfg.Memory.MaxPct))
		return nil
	},
}

var thresholdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current thresholds and alerts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		total := config.TotalRAMKB()
		fmt.Printf("Soft threshold:  %.0f%% (%.1f GB)\n", cfg.Memory.SoftPct, gbpct(total, cfg.Memory.SoftPct))
		fmt.Printf("Hard threshold:  %.0f%% (%.1f GB)\n", cfg.Memory.HardPct, gbpct(total, cfg.Memory.HardPct))
		fmt.Printf("Kernel max:      %.0f%% (%.1f GB)\n", cfg.Memory.MaxPct, gbpct(total, cfg.Memory.MaxPct))
		fmt.Printf("Temp kill:       %.0f C\n", cfg.Temperature.Critical)
		if cfg.Network.AlertMBps > 0 {
			fmt.Printf("Network alert:   %.1f MB/s\n", cfg.Network.AlertMBps)
		}
		if cfg.CPU.AlertPct > 0 {
			fmt.Printf("CPU alert:       %.1f%%\n", cfg.CPU.AlertPct)
		}
		if cfg.Disk.SpaceAlertPct > 0 {
			fmt.Printf("Disk space alert: %.1f%%\n", cfg.Disk.SpaceAlertPct)
		}
		fmt.Printf("Kill policy:     %s (cooldown %s, max %d/min)\n", cfg.Kill.Policy, cfg.Kill.Cooldown, cfg.Kill.MaxPerMin)
		fmt.Printf("Protect:         %v\n", cfg.Protect)
		fmt.Printf("Expendable:      %v\n", cfg.Expendable)
		fmt.Printf("Config:          %s\n", config.Path())
		return nil
	},
}

var thresholdCmd = &cobra.Command{
	Use:   "threshold",
	Short: "Manage memory thresholds",
}

func init() {
	thresholdSetCmd.Flags().Float64("soft", 0, "Soft threshold in GB (notification)")
	thresholdSetCmd.Flags().Float64("hard", 0, "Hard threshold in GB (auto-kill)")
	thresholdSetCmd.Flags().Float64("max", 0, "Kernel safety-net limit in GB")
	thresholdSetCmd.Flags().Float64("soft-pct", 90, "Soft threshold as % of total RAM")
	thresholdSetCmd.Flags().Float64("hard-pct", 96, "Hard threshold as % of total RAM")
	thresholdSetCmd.Flags().Float64("max-pct", 99, "Kernel max as % of total RAM")
	thresholdSetCmd.Flags().Float64("net-alert", 0, "Notify if any interface exceeds this rx+tx (MB/s, 0 = off)")
	thresholdSetCmd.Flags().Float64("cpu-alert", 0, "Notify if overall CPU usage exceeds this percent (0 = off)")
	thresholdSetCmd.Flags().Float64("disk-alert", 0, "Notify if any mount exceeds this used percent (0 = off)")
	thresholdSetCmd.Flags().Float64("temp-kill", 0, "Kill top consumer above this temperature (C, 0 = off)")

	thresholdCmd.AddCommand(thresholdSetCmd)
	thresholdCmd.AddCommand(thresholdStatusCmd)
}
