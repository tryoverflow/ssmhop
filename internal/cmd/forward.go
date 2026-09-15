package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/spf13/cobra"

	"github.com/tryoverflow/ssmhop/internal/session"
	"github.com/tryoverflow/ssmhop/internal/target"
)

const (
	docPortForward       = "AWS-StartPortForwardingSession"
	docPortForwardRemote = "AWS-StartPortForwardingSessionToRemoteHost"
)

func forwardCmd(g *globalOpts) *cobra.Command {
	var (
		localPort string
		remote    string
	)

	cmd := &cobra.Command{
		Use:   "forward [filter]",
		Short: "Forward a local port through an EC2 instance or ECS container",
		Long: `Forward a local port over SSM, tunnelling either to a port on the target
itself or onward to any host the target can reach (an RDS endpoint, an internal
load balancer, a service in the same VPC).

The target can be an EC2 instance or an ECS container — both are addressable as
SSM targets, so both accept the AWS port-forwarding documents. Run without
arguments to pick from everything available.

ECS targets need enableExecuteCommand on the task, the same prerequisite as
"ssmhop exec".`,
		Example: `  # to a port on the target itself
  ssmhop forward --remote 8080

  # onward to a private RDS instance, listening locally on 15432
  ssmhop forward --remote db.internal:5432 --local 15432

  # restrict the picker to ECS containers in the prod cluster
  ssmhop forward api --kind ecs --cluster prod --remote 9000`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host, remotePort, err := parseRemote(remote)
			if err != nil {
				return err
			}
			if localPort == "" {
				localPort = remotePort
			}
			if err := validatePort(localPort); err != nil {
				return fmt.Errorf("--local: %w", err)
			}

			t, err := selectTarget(cmd, g, []target.Kind{target.KindEC2, target.KindECS}, args, "target")
			if err != nil {
				return err
			}

			doc := docPortForward
			params := map[string][]string{
				"portNumber":      {remotePort},
				"localPortNumber": {localPort},
			}
			if host != "" {
				doc = docPortForwardRemote
				params["host"] = []string{host}
			}

			c := clientsFrom(cmd)
			out, err := c.SSM.StartSession(cmd.Context(), &ssm.StartSessionInput{
				Target:       aws.String(t.SSMTarget()),
				DocumentName: aws.String(doc),
				Parameters:   params,
			})
			if err != nil {
				return fmt.Errorf("start port forwarding session on %s: %w", t.SSMTarget(), err)
			}

			dest := "localhost:" + remotePort
			if host != "" {
				dest = host + ":" + remotePort
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "forwarding localhost:%s -> %s via %s\n", localPort, dest, t.SSMTarget())

			return session.Run(cmd.Context(),
				session.Session{
					SessionID:  aws.ToString(out.SessionId),
					StreamURL:  aws.ToString(out.StreamUrl),
					TokenValue: aws.ToString(out.TokenValue),
				},
				c.Region, c.Profile,
				pluginInput{Target: t.SSMTarget(), DocumentName: doc, Parameters: params},
			)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&localPort, "local", "L", "", "Local port to listen on (defaults to the remote port)")
	f.StringVarP(&remote, "remote", "R", "", "Remote port, or host:port to reach through the target (required)")
	_ = cmd.MarkFlagRequired("remote")

	return cmd
}

func parseRemote(s string) (host, port string, err error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("--remote is required (a port, or host:port)")
	}

	if strings.HasPrefix(s, "[") {
		end := strings.LastIndex(s, "]")
		if end < 0 || !strings.HasPrefix(s[end+1:], ":") {
			return "", "", fmt.Errorf("--remote %q: bracketed address needs a :port suffix", s)
		}
		host, port = s[1:end], s[end+2:]
		if err := validatePort(port); err != nil {
			return "", "", fmt.Errorf("--remote: %w", err)
		}
		return host, port, nil
	}

	if i := strings.LastIndex(s, ":"); i >= 0 {
		host, port = s[:i], s[i+1:]
		if host == "" {
			return "", "", fmt.Errorf("--remote %q: missing host before the colon", s)
		}

		if strings.Contains(host, ":") {
			return "", "", fmt.Errorf("--remote %q: bracket IPv6 addresses, as in [%s]:PORT", s, s)
		}
		if err := validatePort(port); err != nil {
			return "", "", fmt.Errorf("--remote: %w", err)
		}
		return host, port, nil
	}

	if err := validatePort(s); err != nil {
		return "", "", fmt.Errorf("--remote: %w", err)
	}
	return "", s, nil
}

func validatePort(s string) error {
	n, err := strconv.Atoi(s)
	if err != nil {
		return fmt.Errorf("%q is not a port number", s)
	}
	if n < 1 || n > 65535 {
		return fmt.Errorf("port %d out of range 1-65535", n)
	}
	return nil
}
