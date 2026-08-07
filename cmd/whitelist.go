package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var whitelistCmd = &cobra.Command{
	Use:   "whitelist",
	Short: "Manage the process whitelist",
}

var whitelistAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a process name to the whitelist",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		cfg, err := loadConfig()
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			cfg = Config{}
		}

		for _, n := range cfg.Whitelist {
			if n == name {
				fmt.Printf("%s is already whitelisted\n", name)
				return nil
			}
		}

		cfg.Whitelist = append(cfg.Whitelist, name)
		if err := saveConfig(cfg); err != nil {
			return err
		}
		fmt.Printf("Added %s to whitelist\n", name)
		return nil
	},
}

var whitelistRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a process name from the whitelist",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		cfg, err := loadConfig()
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			cfg = Config{}
		}

		for i, n := range cfg.Whitelist {
			if n == name {
				cfg.Whitelist = append(cfg.Whitelist[:i], cfg.Whitelist[i+1:]...)
				if err := saveConfig(cfg); err != nil {
					return err
				}
				fmt.Printf("Removed %s from whitelist\n", name)
				return nil
			}
		}
		fmt.Printf("%s is not in the whitelist\n", name)
		return nil
	},
}

var whitelistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List whitelisted process names",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := loadConfig()
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			cfg = Config{}
		}

		if len(cfg.Whitelist) == 0 {
			fmt.Println("Whitelist is empty")
			return nil
		}
		for _, n := range cfg.Whitelist {
			fmt.Println(n)
		}
		return nil
	},
}

func init() {
	whitelistAddCmd.Flags().String("name", "", "Process name to add")
	whitelistRemoveCmd.Flags().String("name", "", "Process name to remove")

	whitelistCmd.AddCommand(whitelistAddCmd)
	whitelistCmd.AddCommand(whitelistRemoveCmd)
	whitelistCmd.AddCommand(whitelistListCmd)
	rootCmd.AddCommand(whitelistCmd)
}
