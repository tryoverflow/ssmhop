package cmd

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/spf13/cobra"

	"github.com/tryoverflow/ssmhop/internal/session"
	"github.com/tryoverflow/ssmhop/internal/target"
)

func execCmd(g *globalOpts) *cobra.Command {
	var command string

	cmd := &cobra.Command{
		Use:   "exec [filter]",
		Short: "Run a command or shell inside an ECS container (ECS Exec)",
		Long: `Attach to a container in a running ECS task via ECS Exec, which is itself
built on SSM Session Manager.

Run without arguments to pick from every container in every running task, or
pass a filter to narrow by service, container, cluster or task ID. Use
--cluster to avoid walking every cluster on a busy account.

The task must have been started with enableExecuteCommand. That cannot be
switched on for a task already running — the service needs a new deployment:

  aws ecs update-service --cluster CLUSTER --service SERVICE \
    --enable-execute-command --force-new-deployment`,
		Example: "  ssmhop exec\n  ssmhop exec api --cluster prod\n  ssmhop exec api --command \"ps aux\"",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			t, err := selectTarget(cmd, g, []target.Kind{target.KindECS}, args, "container")
			if err != nil {
				return err
			}

			c := clientsFrom(cmd)
			out, err := c.ECS.ExecuteCommand(cmd.Context(), &ecs.ExecuteCommandInput{
				Cluster:     aws.String(t.Cluster),
				Task:        aws.String(t.TaskARN),
				Container:   aws.String(t.Container),
				Command:     aws.String(command),
				Interactive: true,
			})
			if err != nil {
				return fmt.Errorf("execute-command on %s/%s: %w", t.Cluster, t.Container, err)
			}
			if out.Session == nil {
				return fmt.Errorf("ECS returned no session for %s", t.SSMTarget())
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "exec into %s/%s (%s)\n", t.Service, t.Container, t.TaskID)

			return session.Run(cmd.Context(),
				session.Session{
					SessionID:  aws.ToString(out.Session.SessionId),
					StreamURL:  aws.ToString(out.Session.StreamUrl),
					TokenValue: aws.ToString(out.Session.TokenValue),
				},
				c.Region, c.Profile,

				pluginInput{Target: t.SSMTarget()},
			)
		},
	}

	cmd.Flags().StringVar(&command, "command", "/bin/sh", "Command to run in the container")
	return cmd
}
