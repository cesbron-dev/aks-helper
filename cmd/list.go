package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newListCmd() *cobra.Command {
	var (
		plain  bool
		asJSON bool
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List stored clusters",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := store()
			if err != nil {
				return err
			}
			entries, err := st.List()
			if err != nil {
				return err
			}
			current, _ := st.Current()

			if asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			if plain {
				for _, e := range entries {
					fmt.Println(e.Name)
				}
				return nil
			}

			if len(entries) == 0 {
				fmt.Fprintln(os.Stderr, "no clusters stored — run 'aks-helper sync'")
				return nil
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "\tNAME\tSUBSCRIPTION\tRESOURCE GROUP\tLOGIN")
			for _, e := range entries {
				marker := " "
				if e.Name == current {
					marker = "*"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", marker, e.Name, dash(e.Subscription), dash(e.ResourceGroup), dash(e.LoginMode))
			}
			return tw.Flush()
		},
	}

	cmd.Flags().BoolVar(&plain, "plain", false, "print only names, one per line (script-friendly)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print full metadata as JSON")
	return cmd
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
