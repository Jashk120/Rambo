package cmd

import (
	"fmt"
	"os"

	"github.com/jashk120/rambo/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rambo",
	Short: "Event-driven system monitor with auto-kill",
	Long: "Rambo watches RAM (via cgroup v2 limits), temperature, pressure, " +
		"network, CPU and disk using kernel-backed sources, and acts on " +
		"configurable policies.",
	Version: version.Version,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("rambo %s\n", version.String())
	},
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
	rootCmd.AddCommand(versionCmd)
	// cobra's built-in --version flag
	rootCmd.SetVersionTemplate("rambo {{.Version}}\n")
}
