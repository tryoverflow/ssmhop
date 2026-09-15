package picker

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/tryoverflow/ssmhop/internal/target"
)

func mkEC2(name, id, ping string) target.Target {
	return target.Target{Kind: target.KindEC2, InstanceID: id, Name: name,
		Platform: "Linux", AgentVersion: "3.2.1377.0", PingStatus: ping}
}

func mkECS(cluster, service, container, taskID, runtimeID, status string, exec bool, agent string) target.Target {
	return target.Target{Kind: target.KindECS, Cluster: cluster, Service: service,
		Container: container, TaskID: taskID, RuntimeID: runtimeID,
		TaskStatus: status, ExecEnabled: exec, ExecAgent: agent}
}

func sampleTargets() []target.Target {
	return []target.Target{
		mkEC2("app-01", "i-0a1b2c3d4e5f60718", "Online"),
		mkEC2("bastion-01", "i-0b2c3d4e5f6071829", "Online"),
		mkECS("prod", "api", "app", "1a2b3c4d", "r1", "RUNNING", true, "RUNNING"),
	}
}

func TestFzfArgsLayout(t *testing.T) {
	args := fzfArgs(Options{Noun: "target", Region: "eu-west-1"})
	joined := strings.Join(args, " ")

	for _, want := range []string{"--layout=reverse", "--header-first"} {
		if !strings.Contains(joined, want) {
			t.Errorf("fzf args missing %q: %v", want, args)
		}
	}
	if !strings.Contains(joined, "--header=region (eu-west-1)") {
		t.Errorf("header not passed to fzf: %v", args)
	}
	if !strings.Contains(joined, "--prompt=? Choose a target in AWS: ") {
		t.Errorf("prompt should carry gossm's leading '? ': %v", args)
	}

	if !strings.Contains(joined, "--with-nth=2..") {
		t.Errorf("index field not hidden: %v", args)
	}

	if strings.Contains(joined, "--preview") {
		t.Errorf("fzf args should not open a preview window: %v", args)
	}
}

func TestFzfInput(t *testing.T) {
	got := fzfInput(sampleTargets())
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), got)
	}

	if want := "0\tapp-01    (i-0a1b2c3d4e5f60718)"; lines[0] != want {
		t.Errorf("line 0\n got %q\nwant %q", lines[0], want)
	}
	if !strings.HasPrefix(lines[2], "2\tprod/api/app") {
		t.Errorf("line 2 = %q, want the ECS path form", lines[2])
	}
}

func TestParseFzfSelection(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		want    int
		wantErr error
	}{
		{name: "first item", out: "0\tapp-01    (i-0a1b2)\n", want: 0},
		{name: "later item", out: "2\tprod/api/app    (1a2b3c4d)\n", want: 2},
		{name: "no trailing newline", out: "1\tx", want: 1},
		{name: "crlf", out: "1\tx\r\n", want: 1},

		{name: "empty", out: "", wantErr: ErrAborted},
		{name: "newline only", out: "\n", wantErr: ErrAborted},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseFzfSelection(tc.out, 3)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("parseFzfSelection(%q) error = %v, want %v", tc.out, err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseFzfSelection(%q): %v", tc.out, err)
			}
			if got != tc.want {
				t.Errorf("parseFzfSelection(%q) = %d, want %d", tc.out, got, tc.want)
			}
		})
	}

	for _, bad := range []string{"9\tx", "-1\tx", "abc\tx", "noTabHere"} {
		if _, err := parseFzfSelection(bad, 3); err == nil {
			t.Errorf("parseFzfSelection(%q) = nil error, want one", bad)
		}
	}
}

func TestPickSelectsLoneTarget(t *testing.T) {
	only := sampleTargets()[:1]
	got, err := Pick(only, Options{Noun: "instance"})
	if err != nil {
		t.Fatalf("Pick: %v", err)
	}
	if got.InstanceID != only[0].InstanceID {
		t.Errorf("Pick returned %q, want %q", got.InstanceID, only[0].InstanceID)
	}
}

func TestFzfPickAgainstRealBinary(t *testing.T) {
	if _, err := exec.LookPath(fzfBinary); err != nil {
		t.Skip("fzf not installed")
	}

	items := sampleTargets()
	idx, err := fzfPick(items, Options{
		Noun: "target", Region: "eu-west-1", Query: "bastion",
	})
	if err != nil {
		if errors.Is(err, errFzfUnusable) {
			t.Fatalf("installed fzf rejected the flags: %v", fzfArgs(Options{}))
		}
		t.Fatalf("fzfPick: %v", err)
	}
	if idx != 1 {
		t.Errorf("fzfPick chose index %d (%s), want 1 (bastion)", idx, items[idx].Label())
	}
}
