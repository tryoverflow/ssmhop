package cmd

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"

	"github.com/tryoverflow/ssmhop/internal/session"
	"github.com/tryoverflow/ssmhop/internal/target"
)

func connectCmd(g *globalOpts) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "connect [filter]",
		Short: "Open an interactive shell on an EC2 instance",
		Long: `Open an interactive shell on an SSM-managed EC2 instance.

Run without arguments to pick from every managed instance, or pass a filter to
narrow the list by Name tag or instance ID first.

For a shell inside an ECS container, use "ssmhop exec" instead.`,
		Example: "  ssmhop connect\n  ssmhop connect web-prod\n  ssmhop connect -p prod -r eu-west-1",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := selectTarget(cmd, g, []target.Kind{target.KindEC2}, args, "instance")
			if err != nil {
				return err
			}

			c := clientsFrom(cmd)
			in := &ssm.StartSessionInput{Target: aws.String(t.SSMTarget())}
			if reason != "" {
				in.Reason = aws.String(reason)
			}

			out, err := c.SSM.StartSession(cmd.Context(), in)
			if err != nil {
				return fmt.Errorf("start session on %s: %w", t.SSMTarget(), err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "connected to %s (%s)\n", t.SSMTarget(), t.Name)

			return session.Run(cmd.Context(),
				session.Session{
					SessionID:  aws.ToString(out.SessionId),
					StreamURL:  aws.ToString(out.StreamUrl),
					TokenValue: aws.ToString(out.TokenValue),
				},
				c.Region, c.Profile,
				pluginInput{Target: t.SSMTarget()},
			)
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Reason recorded in the session audit trail")
	return cmd
}

type pluginInput struct {
	Target       string              `json:"Target"`
	DocumentName string              `json:"DocumentName,omitempty"`
	Parameters   map[string][]string `json:"Parameters,omitempty"`
}
