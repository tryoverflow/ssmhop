package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

const PluginName = "session-manager-plugin"

type Session struct {
	SessionID  string `json:"SessionId"`
	StreamURL  string `json:"StreamUrl"`
	TokenValue string `json:"TokenValue"`
}

func Endpoint(region string) string {
	host := "amazonaws.com"
	if strings.HasPrefix(region, "cn-") {
		host = "amazonaws.com.cn"
	}
	return fmt.Sprintf("https://ssm.%s.%s", region, host)
}

func Run(ctx context.Context, s Session, region, profile string, input any) error {
	if s.StreamURL == "" || s.TokenValue == "" {
		return errors.New("session response was missing StreamUrl or TokenValue")
	}

	validProfile := regexp.MustCompile(`^[a-zA-Z0-9_\-\./\\]+$`)
	if profile != "" && !validProfile.MatchString(profile) {
		return fmt.Errorf("invalid input")
	}

	sessionJSON, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("marshal session input: %w", err)
	}

	pluginPath, err := exec.LookPath(PluginName)
	if err != nil {
		return fmt.Errorf(
			"%s not found in PATH — install it from "+
				"https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-working-with-install-plugin.html: %w",
			PluginName, err)
	}

	cmd := exec.CommandContext(ctx, pluginPath,
		string(sessionJSON),
		region,
		"StartSession",
		profile,
		string(inputJSON),
		Endpoint(region),
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	restore := ignoreTerminalSignals()
	defer restore()

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s exited with error: %w", PluginName, err)
	}
	return nil
}
