package cmd

import (
	"fmt"

	"github.com/jashk120/rambo/internal/config"
	"github.com/spf13/cobra"
)

func protectAddRunE(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for _, n := range cfg.Protect {
		if n == name {
			fmt.Printf("%s is already protected\n", name)
			return nil
		}
	}
	cfg.Protect = append(cfg.Protect, name)
	if err := config.Save(config.Path(), cfg); err != nil {
		return err
	}
	fmt.Printf("Added %s to protect list (never killed)\n", name)
	return nil
}

func protectRemoveRunE(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return fmt.Errorf("--name is required")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	for i, n := range cfg.Protect {
		if n == name {
			cfg.Protect = append(cfg.Protect[:i], cfg.Protect[i+1:]...)
			if err := config.Save(config.Path(), cfg); err != nil {
				return err
			}
			fmt.Printf("Removed %s from protect list\n", name)
			return nil
		}
	}
	fmt.Printf("%s is not in the protect list\n", name)
	return nil
}

func protectListRunE(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if len(cfg.Protect) == 0 {
		fmt.Println("Protect list is empty")
		return nil
	}
	for _, n := range cfg.Protect {
		fmt.Println(n)
	}
	return nil
}

var protectCmd = &cobra.Command{
	Use:   "protect",
	Short: "Manage the process protect list (never killed)",
}

var protectAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a process name to the protect list",
	RunE:  protectAddRunE,
}

var protectRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a process name from the protect list",
	RunE:  protectRemoveRunE,
}

var protectListCmd = &cobra.Command{
	Use:   "list",
	Short: "List protected process names",
	RunE:  protectListRunE,
}

// whitelist is kept as a backwards-compatible alias for protect.
var whitelistCmd = &cobra.Command{
	Use:    "whitelist",
	Short:  "Alias for protect",
	Hidden: true,
}

var whitelistAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add a process name to the protect list",
	RunE:  protectAddRunE,
}

var whitelistRemoveCmd = &cobra.Command{
	Use:   "remove",
	Short: "Remove a process name from the protect list",
	RunE:  protectRemoveRunE,
}

var whitelistListCmd = &cobra.Command{
	Use:   "list",
	Short: "List protected process names",
	RunE:  protectListRunE,
}

func init() {
	for _, c := range []*cobra.Command{protectAddCmd, protectRemoveCmd, protectListCmd, whitelistAddCmd, whitelistRemoveCmd, whitelistListCmd} {
		c.Flags().String("name", "", "Process name")
	}
	protectCmd.AddCommand(protectAddCmd)
	protectCmd.AddCommand(protectRemoveCmd)
	protectCmd.AddCommand(protectListCmd)
	whitelistCmd.AddCommand(whitelistAddCmd)
	whitelistCmd.AddCommand(whitelistRemoveCmd)
	whitelistCmd.AddCommand(whitelistListCmd)
}
