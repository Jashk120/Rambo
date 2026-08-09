package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rambo",
	Short: "Event-driven system monitor with auto-kill",
	Long: "Rambo watches RAM (via cgroup v2 limits), temperature, pressure, " +
		"network, CPU and disk using kernel-backed sources, and acts on " +
		"configurable policies.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(daemonCmd)
	rootCmd.AddCommand(thresholdCmd)
	rootCmd.AddCommand(protectCmd)
	rootCmd.AddCommand(whitelistCmd)
	rootCmd.AddCommand(historyCmd)
	rootCmd.AddCommand(topCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(cleanCmd)
}
