package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/jashk120/rambo/internal/state"
	"github.com/spf13/cobra"
)

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show recent events and kills",
	RunE: func(cmd *cobra.Command, args []string) error {
		source, _ := cmd.Flags().GetString("source")
		n, _ := cmd.Flags().GetInt("n")

		recs, err := state.History(source, n)
		if err != nil {
			if len(recs) == 0 {
				fmt.Println("No history yet")
				return nil
			}
			return err
		}
		if len(recs) == 0 {
			fmt.Println("No history yet")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIME\tSOURCE\tSEVERITY\tACTION\tPROCESS\tMESSAGE")
		for _, r := range recs {
			proc := ""
			if r.Process != "" {
				proc = fmt.Sprintf("%s(%d)", r.Process, r.PID)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Time, r.Source, r.Severity, r.Action, proc, r.Message)
		}
		return w.Flush()
	},
}

func init() {
	historyCmd.Flags().String("source", "", "Filter by source (memory, temperature, network, ...)")
	historyCmd.Flags().Int("n", 20, "Number of records to show (0 = all)")
}
