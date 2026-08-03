package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

type Config struct {
	SoftGB float64 `json:"soft_gb"`
	HardGB float64 `json:"hard_gb"`
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

// threshold set --soft 7.5 --hard 7.9
var thresholdSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set soft and hard RAM thresholds",
	RunE: func(cmd *cobra.Command, args []string) error {
		soft, _ := cmd.Flags().GetFloat64("soft")
		hard, _ := cmd.Flags().GetFloat64("hard")

		if soft >= hard {
			return fmt.Errorf("soft threshold must be less than hard threshold")
		}

		cfg := Config{SoftGB: soft, HardGB: hard}
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
	thresholdSetCmd.MarkFlagRequired("soft")
	thresholdSetCmd.MarkFlagRequired("hard")

	// wire up: root → threshold → set/status
	thresholdCmd.AddCommand(thresholdSetCmd)
	thresholdCmd.AddCommand(thresholdStatusCmd)
	rootCmd.AddCommand(thresholdCmd)
}
