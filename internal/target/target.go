package target

import (
	"fmt"
	"strings"
)

type Kind string

const (
	KindEC2 Kind = "ec2"
	KindECS Kind = "ecs"
)

type Target struct {
	Kind Kind

	InstanceID   string
	Name         string
	Platform     string
	AgentVersion string
	PingStatus   string

	Cluster     string
	Service     string
	TaskID      string
	TaskARN     string
	Container   string
	RuntimeID   string
	TaskStatus  string
	ExecEnabled bool
	ExecAgent   string
}

func (t Target) SSMTarget() string {
	if t.Kind == KindECS {
		return fmt.Sprintf("ecs:%s_%s_%s", t.Cluster, t.TaskID, t.RuntimeID)
	}
	return t.InstanceID
}

func (t Target) Ready() (bool, string) {
	if t.Kind == KindECS {
		switch {
		case !t.ExecEnabled:
			return false, "task was not started with enableExecuteCommand — needs a new deployment"
		case !strings.EqualFold(t.TaskStatus, "RUNNING"):
			return false, "task is " + t.TaskStatus
		case t.RuntimeID == "":
			return false, "container has no runtimeId yet"
		case t.ExecAgent != "" && !strings.EqualFold(t.ExecAgent, "RUNNING"):
			return false, "ExecuteCommandAgent is " + t.ExecAgent
		}
		return true, ""
	}
	if !strings.EqualFold(t.PingStatus, "Online") {
		return false, "SSM agent is " + t.PingStatus
	}
	return true, ""
}

func (t Target) DisplayName() string {
	if t.Kind != KindECS {
		return nonEmpty(t.Name, t.InstanceID)
	}
	parts := make([]string, 0, 3)
	for _, p := range []string{t.Cluster, t.Service, t.Container} {
		if strings.TrimSpace(p) != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 {
		return t.TaskID
	}
	return strings.Join(parts, "/")
}

func (t Target) DisplayID() string {
	if t.Kind == KindECS {
		return t.TaskID
	}
	return t.InstanceID
}

func (t Target) Label() string {
	label := fmt.Sprintf("%s    (%s)", t.DisplayName(), t.DisplayID())
	if tag := t.statusTag(); tag != "" {
		label += "    " + tag
	}
	return label
}

func (t Target) statusTag() string {
	if ok, _ := t.Ready(); ok {
		return ""
	}
	if t.Kind == KindECS {
		switch {
		case !t.ExecEnabled:
			return "[exec not enabled]"
		case !strings.EqualFold(t.TaskStatus, "RUNNING"):
			return "[" + t.TaskStatus + "]"
		case t.RuntimeID == "":
			return "[starting]"
		default:
			return "[agent " + t.ExecAgent + "]"
		}
	}
	return "[" + nonEmpty(t.PingStatus, "unknown") + "]"
}

func (t Target) Matches(filter string) bool {
	if filter == "" {
		return true
	}
	f := strings.ToLower(filter)
	for _, field := range []string{
		t.Name, t.InstanceID, t.Cluster, t.Service, t.Container, t.TaskID,
	} {
		if field != "" && strings.Contains(strings.ToLower(field), f) {
			return true
		}
	}
	return false
}

func nonEmpty(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}
