package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "rambo",
	Short: "Userspace RAM manager with auto-kill",
	Long:  "Rambo monitors RAM usage, notifies on soft threshold, and auto-kills on hard threshold.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
