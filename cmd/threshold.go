package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type Config struct {
	SoftGB       float64  `json:"soft_gb"`
	HardGB       float64  `json:"hard_gb"`
	NetAlertMbps float64  `json:"net_alert_mbps,omitempty"`
	CPUAlertPct  float64  `json:"cpu_alert_pct,omitempty"`
	DiskAlertPct float64  `json:"disk_alert_pct,omitempty"`
	Whitelist    []string `json:"whitelist,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "rambo", "config.json")
}

func saveConfig(cfg Config) error {
	path := configPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func loadConfig() (Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return cfg, err
}

// threshold set --soft 7.5 --hard 7.9 [--net-alert 900 --cpu-alert 90 --disk-alert 90]
var thresholdSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set soft and hard RAM thresholds",
	RunE: func(cmd *cobra.Command, args []string) error {
		soft, _ := cmd.Flags().GetFloat64("soft")
		hard, _ := cmd.Flags().GetFloat64("hard")

		if soft >= hard {
			return fmt.Errorf("soft threshold must be less than hard threshold")
		}

		cfg, err := loadConfig()
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			cfg = Config{}
		}
		cfg.SoftGB = soft
		cfg.HardGB = hard

		if cmd.Flags().Changed("net-alert") {
			cfg.NetAlertMbps, _ = cmd.Flags().GetFloat64("net-alert")
		}
		if cmd.Flags().Changed("cpu-alert") {
			cfg.CPUAlertPct, _ = cmd.Flags().GetFloat64("cpu-alert")
		}
		if cmd.Flags().Changed("disk-alert") {
			cfg.DiskAlertPct, _ = cmd.Flags().GetFloat64("disk-alert")
		}

		if err := saveConfig(cfg); err != nil {
			return err
		}

		fmt.Printf("Thresholds set — soft: %.1f GB, hard: %.1f GB\n", soft, hard)
		return nil
	},
}

// threshold status
var thresholdStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current thresholds",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			return fmt.Errorf("no config found, run 'rambo threshold set' first")
		}
		fmt.Printf("Soft threshold: %.1f GB\n", cfg.SoftGB)
		fmt.Printf("Hard threshold: %.1f GB\n", cfg.HardGB)
		if cfg.NetAlertMbps > 0 {
			fmt.Printf("Network alert:  %.1f MB/s\n", cfg.NetAlertMbps)
		}
		if cfg.CPUAlertPct > 0 {
			fmt.Printf("CPU alert:      %.1f%%\n", cfg.CPUAlertPct)
		}
		if cfg.DiskAlertPct > 0 {
			fmt.Printf("Disk alert:     %.1f%%\n", cfg.DiskAlertPct)
		}
		if len(cfg.Whitelist) > 0 {
			fmt.Printf("Whitelist:      %v\n", cfg.Whitelist)
		}
		return nil
	},
}

// threshold (parent command)
var thresholdCmd = &cobra.Command{
	Use:   "threshold",
	Short: "Manage RAM thresholds",
}

func init() {
	// flags on the set subcommand
	thresholdSetCmd.Flags().Float64("soft", 0, "Soft threshold in GB (notification)")
	thresholdSetCmd.Flags().Float64("hard", 0, "Hard threshold in GB (auto-kill)")
	thresholdSetCmd.Flags().Float64("net-alert", 0, "Notify if any interface exceeds this rx+tx (MB/s, 0 = off)")
	thresholdSetCmd.Flags().Float64("cpu-alert", 0, "Notify if overall CPU usage exceeds this percent (0 = off)")
	thresholdSetCmd.Flags().Float64("disk-alert", 0, "Notify if any mount exceeds this used percent (0 = off)")
	thresholdSetCmd.MarkFlagRequired("soft")
	thresholdSetCmd.MarkFlagRequired("hard")

	// wire up: root → threshold → set/status
	thresholdCmd.AddCommand(thresholdSetCmd)
	thresholdCmd.AddCommand(thresholdStatusCmd)
	rootCmd.AddCommand(thresholdCmd)
}
