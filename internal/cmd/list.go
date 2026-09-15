package cmd

import (
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/tryoverflow/ssmhop/internal/target"
)

func listCmd(g *globalOpts) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "list [filter]",
		Short: "List every SSM-reachable EC2 instance and ECS container",
		Long: `List targets a session could attach to: SSM-managed EC2 instances and
containers in running ECS tasks.

The READY column is the useful one — it tells you whether a session would
actually connect, and why not when it would not.`,
		Example: "  ssmhop list\n  ssmhop list --kind ecs --cluster prod\n  ssmhop list -o json",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filter := ""
			if len(args) > 0 {
				filter = args[0]
			}

			targets, err := discover(cmd, g, []target.Kind{target.KindEC2, target.KindECS}, filter)
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()
			if asJSON {
				if targets == nil {
					targets = []target.Target{}
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(targets)
			}

			if len(targets) == 0 {
				fmt.Fprintln(out, "no targets found")
				return nil
			}

			w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
			fmt.Fprintln(w, "KIND\tNAME\tCONTAINER\tID\tSTATUS\tREADY")
			for _, t := range targets {
				ready, why := t.Ready()
				readyCol := "yes"
				if !ready {
					readyCol = "no — " + why
				}

				switch t.Kind {
				case target.KindEC2:
					fmt.Fprintf(w, "ec2\t%s\t%s\t%s\t%s\t%s\n",
						dash(t.Name), "-", t.InstanceID, t.PingStatus, readyCol)
				case target.KindECS:
					fmt.Fprintf(w, "ecs\t%s\t%s\t%s\t%s\t%s\n",
						dash(t.Service), dash(t.Container), t.TaskID, t.TaskStatus, readyCol)
				}
			}
			return w.Flush()
		},
	}

	cmd.Flags().BoolVarP(&asJSON, "json", "o", false, "Emit JSON instead of a table")
	return cmd
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
