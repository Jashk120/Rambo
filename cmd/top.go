package cmd

import (
	"fmt"

	"github.com/jashk120/rambo/internal/monitor"
	"github.com/spf13/cobra"
)

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Show top RAM consuming processes",
	RunE: func(cmd *cobra.Command, args []string) error {
		mem, err := monitor.GetMemInfo()
		if err != nil {
			return err
		}

		fmt.Printf("RAM: %d MB used / %d MB total\n\n",
			mem.Used/1024, mem.Total/1024)

		procs, err := monitor.GetTopProcesses(10)
		if err != nil {
			return err
		}

		fmt.Printf("%-8s %-20s %s\n", "PID", "NAME", "RAM")
		fmt.Println("---------------------------------------")
		for _, p := range procs {
			fmt.Printf("%-8d %-20s %d MB\n", p.PID, p.Name, p.RSS/1024)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(topCmd)
}
