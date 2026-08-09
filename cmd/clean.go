package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete the rambo log file",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		path := filepath.Join(home, ".local", "share", "rambo", "rambo.log")

		if err := os.Remove(path); err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No log file to clean")
				return nil
			}
			return err
		}
		fmt.Printf("Removed %s\n", path)
		return nil
	},
}
