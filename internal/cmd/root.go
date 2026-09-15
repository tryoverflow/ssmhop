package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tryoverflow/ssmhop/internal/awsx"
	"github.com/tryoverflow/ssmhop/internal/picker"
	"github.com/tryoverflow/ssmhop/internal/target"
)

var Version = "dev"

type globalOpts struct {
	profile string
	region  string
	cluster string
	kind    string
	noFzf   bool
}

type ctxKey struct{}

func clientsFrom(cmd *cobra.Command) *awsx.Clients {
	return cmd.Context().Value(ctxKey{}).(*awsx.Clients)
}

func ExecuteContext(ctx context.Context) error {
	g := &globalOpts{}

	root := &cobra.Command{
		Use:   "ssmhop",
		Short: "Interactive AWS SSM sessions and port forwarding for EC2 and ECS",
		Long: `ssmhop gives you shells and port forwards over AWS Systems Manager, for both
EC2 instances and ECS containers, with an interactive picker instead of IDs.

No bastion, no open inbound ports, no SSH keys. Every command works the same
way: run it with no target and pick from a fuzzy list, or pass a filter to
narrow the list first.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if cmd.Name() == "completion" || cmd.Name() == "help" || cmd.Name() == "version" {
				return nil
			}
			clients, err := awsx.Load(cmd.Context(), g.profile, g.region)
			if err != nil {
				return err
			}
			cmd.SetContext(context.WithValue(cmd.Context(), ctxKey{}, clients))
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVarP(&g.profile, "profile", "p", "", "AWS profile")
	pf.StringVarP(&g.region, "region", "r", "", "AWS region")
	pf.StringVarP(&g.cluster, "cluster", "c", "", "Limit ECS discovery to clusters matching this substring")
	pf.StringVar(&g.kind, "kind", "", "Restrict targets to \"ec2\" or \"ecs\"")
	pf.BoolVar(&g.noFzf, "no-fzf", false, "Use the built-in finder even if fzf is installed")

	root.AddCommand(
		connectCmd(g),
		execCmd(g),
		forwardCmd(g),
		listCmd(g),
		versionCmd(),
		completionCmd(),
	)

	return root.ExecuteContext(ctx)
}

func discover(cmd *cobra.Command, g *globalOpts, kinds []target.Kind, filter string) ([]target.Target, error) {
	c := clientsFrom(cmd)
	ctx := cmd.Context()

	if g.kind != "" {
		switch g.kind {
		case string(target.KindEC2):
			kinds = []target.Kind{target.KindEC2}
		case string(target.KindECS):
			kinds = []target.Kind{target.KindECS}
		default:
			return nil, fmt.Errorf("--kind must be ec2 or ecs, got %q", g.kind)
		}
	}

	var all []target.Target
	var errs []error
	for _, k := range kinds {
		switch k {
		case target.KindEC2:
			got, err := target.ListEC2(ctx, c.SSM, c.EC2, filter)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			all = append(all, got...)
		case target.KindECS:
			got, err := target.ListECS(ctx, c.ECS, g.cluster, filter)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			all = append(all, got...)
		}
	}

	if len(all) == 0 && len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	for _, err := range errs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}
	return all, nil
}

func selectTarget(cmd *cobra.Command, g *globalOpts, kinds []target.Kind, args []string, noun string) (target.Target, error) {
	filter := ""
	if len(args) > 0 {
		filter = args[0]
	}

	targets, err := discover(cmd, g, kinds, filter)
	if err != nil {
		return target.Target{}, err
	}
	if len(targets) == 0 {
		return target.Target{}, fmt.Errorf("no targets found (filter %q)", filter)
	}

	c := clientsFrom(cmd)
	t, err := picker.Pick(targets, picker.Options{
		Noun:    noun,
		Region:  c.Region,
		Profile: c.Profile,
		NoFzf:   g.noFzf,
	})
	if err != nil {
		return target.Target{}, err
	}
	if ok, why := t.Ready(); !ok {
		return target.Target{}, fmt.Errorf("%s is not connectable: %s", t.SSMTarget(), why)
	}
	return t, nil
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), Version)
			return err
		},
	}
}

func completionCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completion [bash|zsh|fish|powershell]",
		Short:     "Generate a shell completion script",
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			out := cmd.OutOrStdout()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(out, true)
			case "zsh":
				return root.GenZshCompletion(out)
			case "fish":
				return root.GenFishCompletion(out, true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(out)
			}
			return fmt.Errorf("unsupported shell %q", args[0])
		},
	}
}
