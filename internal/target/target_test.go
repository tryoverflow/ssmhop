package target

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSSMTarget(t *testing.T) {
	ecs := Target{
		Kind:      KindECS,
		Cluster:   "prod-cluster",
		TaskID:    "1a2b3c4d5e6f7890abcdef1234567890",
		RuntimeID: "1a2b3c4d5e6f7890abcdef1234567890-0987654321",
	}
	want := "ecs:prod-cluster_1a2b3c4d5e6f7890abcdef1234567890_1a2b3c4d5e6f7890abcdef1234567890-0987654321"
	if got := ecs.SSMTarget(); got != want {
		t.Errorf("ECS SSMTarget()\n got %q\nwant %q", got, want)
	}

	ec2 := Target{Kind: KindEC2, InstanceID: "i-0abc1234def5678ab", Name: "web-1"}
	if got := ec2.SSMTarget(); got != "i-0abc1234def5678ab" {
		t.Errorf("EC2 SSMTarget() = %q, want the bare instance ID", got)
	}
}

func TestReady(t *testing.T) {
	tests := []struct {
		name      string
		target    Target
		wantReady bool
	}{
		{
			name:      "online instance",
			target:    Target{Kind: KindEC2, PingStatus: "Online"},
			wantReady: true,
		},
		{
			name:      "instance with lost agent",
			target:    Target{Kind: KindEC2, PingStatus: "ConnectionLost"},
			wantReady: false,
		},
		{
			name: "healthy exec-enabled container",
			target: Target{Kind: KindECS, ExecEnabled: true, TaskStatus: "RUNNING",
				RuntimeID: "abc-123", ExecAgent: "RUNNING"},
			wantReady: true,
		},
		{
			name: "container without execute-command",
			target: Target{Kind: KindECS, ExecEnabled: false, TaskStatus: "RUNNING",
				RuntimeID: "abc-123", ExecAgent: "RUNNING"},
			wantReady: false,
		},
		{
			name: "task still provisioning",
			target: Target{Kind: KindECS, ExecEnabled: true, TaskStatus: "PENDING",
				RuntimeID: "abc-123", ExecAgent: "RUNNING"},
			wantReady: false,
		},
		{
			name: "container has no runtime id yet",
			target: Target{Kind: KindECS, ExecEnabled: true, TaskStatus: "RUNNING",
				RuntimeID: "", ExecAgent: "RUNNING"},
			wantReady: false,
		},
		{
			name: "exec agent not running",
			target: Target{Kind: KindECS, ExecEnabled: true, TaskStatus: "RUNNING",
				RuntimeID: "abc-123", ExecAgent: "STOPPED"},
			wantReady: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ready, why := tc.target.Ready()
			if ready != tc.wantReady {
				t.Fatalf("Ready() = (%t, %q), want ready=%t", ready, why, tc.wantReady)
			}
			if !ready && why == "" {
				t.Error("Ready() returned false with no explanation")
			}
		})
	}
}

func TestMatches(t *testing.T) {
	ec2 := Target{Kind: KindEC2, InstanceID: "i-0abc1234", Name: "web-prod-1"}
	ecs := Target{Kind: KindECS, Cluster: "prod", Service: "api", Container: "app", TaskID: "deadbeef"}

	if !ec2.Matches("") || !ecs.Matches("") {
		t.Error("an empty filter should match everything")
	}
	if !ec2.Matches("WEB") {
		t.Error("filter should be case-insensitive")
	}
	if !ec2.Matches("i-0abc") {
		t.Error("filter should match instance ID prefixes")
	}
	if !ecs.Matches("api") {
		t.Error("filter should match the service name")
	}
	if !ecs.Matches("deadbeef") {
		t.Error("filter should match the task ID")
	}
	if ec2.Matches("database") {
		t.Error("unrelated filter should not match")
	}
}

func TestArnTailAndServiceFromGroup(t *testing.T) {
	if got := arnTail("arn:aws:ecs:eu-west-1:123456789012:task/prod-cluster/abc123"); got != "abc123" {
		t.Errorf("arnTail() = %q, want %q", got, "abc123")
	}
	if got := arnTail("arn:aws:ecs:eu-west-1:123456789012:cluster/prod-cluster"); got != "prod-cluster" {
		t.Errorf("arnTail() = %q, want %q", got, "prod-cluster")
	}
	if got := arnTail("bare-name"); got != "bare-name" {
		t.Errorf("arnTail() should pass through a non-ARN, got %q", got)
	}

	if got := serviceFromGroup("service:api"); got != "api" {
		t.Errorf("serviceFromGroup() = %q, want %q", got, "api")
	}
	if got := serviceFromGroup("family:batch-job"); got != "family:batch-job" {
		t.Errorf("serviceFromGroup() should pass through a standalone task group, got %q", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Errorf("truncate() shortened a string that fit: %q", got)
	}
	if got := truncate("averyveryverylongname", 8); len([]rune(got)) != 8 {
		t.Errorf("truncate() = %q, want 8 runes", got)
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate(\"\") = %q", got)
	}

	if got := truncate("wörker-über-lang-name", 8); !utf8.ValidString(got) {
		t.Errorf("truncate() produced invalid UTF-8: %q", got)
	}
	if got := truncate("日本語のインスタンス名", 5); len([]rune(got)) != 5 || !utf8.ValidString(got) {
		t.Errorf("truncate() = %q (%d runes), want 5 valid runes", got, len([]rune(got)))
	}
}

func TestLabel(t *testing.T) {
	ec2 := Target{Kind: KindEC2, InstanceID: "i-0a1b2c3d4e5f60718",
		Name: "app-01", Platform: "Linux", PingStatus: "Online"}
	if got, want := ec2.Label(), "app-01    (i-0a1b2c3d4e5f60718)"; got != want {
		t.Errorf("Label()\n got %q\nwant %q", got, want)
	}

	noName := Target{Kind: KindEC2, InstanceID: "i-0abc1234", PingStatus: "Online"}
	if got, want := noName.Label(), "i-0abc1234    (i-0abc1234)"; got != want {
		t.Errorf("Label() without a Name tag\n got %q\nwant %q", got, want)
	}

	ecs := Target{Kind: KindECS, Cluster: "prod", Service: "api", Container: "app",
		TaskID: "1a2b3c4d", RuntimeID: "r1", TaskStatus: "RUNNING",
		ExecEnabled: true, ExecAgent: "RUNNING"}
	if got, want := ecs.Label(), "prod/api/app    (1a2b3c4d)"; got != want {
		t.Errorf("Label() for ECS\n got %q\nwant %q", got, want)
	}

	standalone := Target{Kind: KindECS, Cluster: "prod", Container: "app",
		TaskID: "1a2b3c4d", RuntimeID: "r1", TaskStatus: "RUNNING",
		ExecEnabled: true, ExecAgent: "RUNNING"}
	if got, want := standalone.Label(), "prod/app    (1a2b3c4d)"; got != want {
		t.Errorf("Label() for a standalone task\n got %q\nwant %q", got, want)
	}
}

func TestLabelFlagsUnreachable(t *testing.T) {
	tests := []struct {
		name   string
		target Target
		want   string
	}{
		{
			name:   "offline agent",
			target: Target{Kind: KindEC2, InstanceID: "i-0abc", Name: "web", PingStatus: "ConnectionLost"},
			want:   "[ConnectionLost]",
		},
		{
			name: "exec not enabled",
			target: Target{Kind: KindECS, Cluster: "prod", Container: "app", TaskID: "t1",
				TaskStatus: "RUNNING", RuntimeID: "r1"},
			want: "[exec not enabled]",
		},
		{
			name: "task pending",
			target: Target{Kind: KindECS, Cluster: "prod", Container: "app", TaskID: "t1",
				TaskStatus: "PENDING", RuntimeID: "r1", ExecEnabled: true},
			want: "[PENDING]",
		},
		{
			name: "no runtime id yet",
			target: Target{Kind: KindECS, Cluster: "prod", Container: "app", TaskID: "t1",
				TaskStatus: "RUNNING", ExecEnabled: true},
			want: "[starting]",
		},
		{
			name: "exec agent stopped",
			target: Target{Kind: KindECS, Cluster: "prod", Container: "app", TaskID: "t1",
				TaskStatus: "RUNNING", RuntimeID: "r1", ExecEnabled: true, ExecAgent: "STOPPED"},
			want: "[agent STOPPED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.target.Label()
			if !strings.HasSuffix(got, tc.want) {
				t.Errorf("Label() = %q, want it to end with %q", got, tc.want)
			}
		})
	}

	ok := Target{Kind: KindEC2, InstanceID: "i-0abc", Name: "web", PingStatus: "Online"}
	if strings.Contains(ok.Label(), "[") {
		t.Errorf("a ready target should carry no tag, got %q", ok.Label())
	}
}

func TestNonEmpty(t *testing.T) {
	if got := nonEmpty("", "-"); got != "-" {
		t.Errorf("nonEmpty(\"\") = %q, want the fallback", got)
	}
	if got := nonEmpty("   ", "-"); got != "-" {
		t.Errorf("nonEmpty(whitespace) = %q, want the fallback", got)
	}
	if got := nonEmpty("web-1", "-"); got != "web-1" {
		t.Errorf("nonEmpty() = %q, want the value", got)
	}
}
