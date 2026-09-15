package target

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type fakeECS struct {
	clusterPages [][]string
	taskPages    map[string][][]string
	tasks        map[string]ecstypes.Task
	failCluster  string
	failClusters bool
	describeSize []int
	listedWith   []ecstypes.DesiredStatus
}

func (f *fakeECS) ListClusters(_ context.Context, in *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	if f.failClusters {
		return nil, errors.New("access denied")
	}
	page, _ := strconv.Atoi(aws.ToString(in.NextToken))
	out := &ecs.ListClustersOutput{ClusterArns: f.clusterPages[page]}
	if page+1 < len(f.clusterPages) {
		out.NextToken = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

func (f *fakeECS) ListTasks(_ context.Context, in *ecs.ListTasksInput, _ ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	cluster := aws.ToString(in.Cluster)
	f.listedWith = append(f.listedWith, in.DesiredStatus)
	if cluster == f.failCluster {
		return nil, errors.New("cluster not found")
	}
	pages := f.taskPages[cluster]
	page, _ := strconv.Atoi(aws.ToString(in.NextToken))
	if page >= len(pages) {
		return &ecs.ListTasksOutput{}, nil
	}
	out := &ecs.ListTasksOutput{TaskArns: pages[page]}
	if page+1 < len(pages) {
		out.NextToken = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

func (f *fakeECS) DescribeTasks(_ context.Context, in *ecs.DescribeTasksInput, _ ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	f.describeSize = append(f.describeSize, len(in.Tasks))
	out := &ecs.DescribeTasksOutput{}
	for _, arn := range in.Tasks {
		if task, ok := f.tasks[arn]; ok {
			out.Tasks = append(out.Tasks, task)
		}
	}
	return out, nil
}

func runningTask(arn, group string, execEnabled bool, containers ...ecstypes.Container) ecstypes.Task {
	return ecstypes.Task{
		TaskArn:              aws.String(arn),
		Group:                aws.String(group),
		LastStatus:           aws.String("RUNNING"),
		EnableExecuteCommand: execEnabled,
		Containers:           containers,
	}
}

func execContainer(name, runtimeID, agentStatus string) ecstypes.Container {
	c := ecstypes.Container{Name: aws.String(name), RuntimeId: aws.String(runtimeID)}
	if agentStatus != "" {
		c.ManagedAgents = []ecstypes.ManagedAgent{{
			Name:       ecstypes.ManagedAgentNameExecuteCommandAgent,
			LastStatus: aws.String(agentStatus),
		}}
	}
	return c
}

func TestListECS(t *testing.T) {
	api := &fakeECS{
		clusterPages: [][]string{
			{"arn:aws:ecs:eu-west-1:123456789012:cluster/prod-web"},
			{"arn:aws:ecs:eu-west-1:123456789012:cluster/staging-web"},
		},
		taskPages: map[string][][]string{
			"prod-web":    {{"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1"}},
			"staging-web": {{"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2"}},
		},
		tasks: map[string]ecstypes.Task{
			"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1": runningTask(
				"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1", "service:api", true,
				execContainer("app", "runtime-app", "RUNNING"),
				execContainer("sidecar", "runtime-side", "RUNNING"),
			),
			"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2": runningTask(
				"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2", "service:api", true,
				execContainer("app", "runtime-stg", "RUNNING"),
			),
		},
	}

	got, err := ListECS(context.Background(), api, "", "")
	if err != nil {
		t.Fatalf("ListECS: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("got %d targets, want 3 (2 containers in prod + 1 in staging): %+v", len(got), got)
	}

	for _, ds := range api.listedWith {
		if ds != ecstypes.DesiredStatusRunning {
			t.Errorf("ListTasks called with DesiredStatus %q, want RUNNING", ds)
		}
	}

	first := got[0]
	if first.Cluster != "prod-web" || first.Service != "api" || first.TaskID != "task1" || first.Container != "app" {
		t.Errorf("unexpected first target: %+v", first)
	}
	if want := "ecs:prod-web_task1_runtime-app"; first.SSMTarget() != want {
		t.Errorf("SSMTarget() = %q, want %q", first.SSMTarget(), want)
	}
	if ok, why := first.Ready(); !ok {
		t.Errorf("healthy container reported not ready: %s", why)
	}
}

func TestListECSClusterFilter(t *testing.T) {
	api := &fakeECS{
		clusterPages: [][]string{{
			"arn:aws:ecs:eu-west-1:123456789012:cluster/prod-web",
			"arn:aws:ecs:eu-west-1:123456789012:cluster/staging-web",
		}},
		taskPages: map[string][][]string{
			"prod-web":    {{"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1"}},
			"staging-web": {{"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2"}},
		},
		tasks: map[string]ecstypes.Task{
			"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1": runningTask(
				"arn:aws:ecs:eu-west-1:123456789012:task/prod-web/task1", "service:api", true,
				execContainer("app", "r1", "RUNNING")),
			"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2": runningTask(
				"arn:aws:ecs:eu-west-1:123456789012:task/staging-web/task2", "service:api", true,
				execContainer("app", "r2", "RUNNING")),
		},
	}

	got, err := ListECS(context.Background(), api, "PROD", "")
	if err != nil {
		t.Fatalf("ListECS: %v", err)
	}
	if len(got) != 1 || got[0].Cluster != "prod-web" {
		t.Fatalf("cluster filter should have matched prod-web case-insensitively, got %+v", got)
	}
}

func TestListECSChunksDescribeTasks(t *testing.T) {
	const total = 250
	arns := make([]string, 0, total)
	tasks := make(map[string]ecstypes.Task, total)
	for i := range total {
		arn := fmt.Sprintf("arn:aws:ecs:eu-west-1:123456789012:task/big/task%d", i)
		arns = append(arns, arn)
		tasks[arn] = runningTask(arn, "service:api", true,
			execContainer("app", fmt.Sprintf("runtime%d", i), "RUNNING"))
	}

	api := &fakeECS{
		clusterPages: [][]string{{"arn:aws:ecs:eu-west-1:123456789012:cluster/big"}},
		taskPages:    map[string][][]string{"big": {arns[:100], arns[100:200], arns[200:]}},
		tasks:        tasks,
	}

	got, err := ListECS(context.Background(), api, "", "")
	if err != nil {
		t.Fatalf("ListECS: %v", err)
	}
	if len(got) != total {
		t.Errorf("got %d targets, want %d", len(got), total)
	}
	if len(api.describeSize) != 3 {
		t.Fatalf("DescribeTasks called %d times with sizes %v, want 3 batches", len(api.describeSize), api.describeSize)
	}
	for _, n := range api.describeSize {
		if n > 100 {
			t.Errorf("DescribeTasks batch of %d exceeds the API limit of 100", n)
		}
	}
}

func TestListECSSkipsBrokenCluster(t *testing.T) {
	api := &fakeECS{
		clusterPages: [][]string{{
			"arn:aws:ecs:eu-west-1:123456789012:cluster/broken",
			"arn:aws:ecs:eu-west-1:123456789012:cluster/good",
		}},
		taskPages: map[string][][]string{
			"good": {{"arn:aws:ecs:eu-west-1:123456789012:task/good/task1"}},
		},
		tasks: map[string]ecstypes.Task{
			"arn:aws:ecs:eu-west-1:123456789012:task/good/task1": runningTask(
				"arn:aws:ecs:eu-west-1:123456789012:task/good/task1", "service:api", true,
				execContainer("app", "r1", "RUNNING")),
		},
		failCluster: "broken",
	}

	got, err := ListECS(context.Background(), api, "", "")
	if err != nil {
		t.Fatalf("ListECS should tolerate one bad cluster, got %v", err)
	}
	if len(got) != 1 || got[0].Cluster != "good" {
		t.Fatalf("got %+v, want the single target from the good cluster", got)
	}
}

func TestListECSPropagatesClusterListFailure(t *testing.T) {
	api := &fakeECS{failClusters: true}
	if _, err := ListECS(context.Background(), api, "", ""); err == nil {
		t.Fatal("ListECS should surface a ListClusters failure")
	}
}

func TestListECSFilterAndNotReady(t *testing.T) {
	arn := "arn:aws:ecs:eu-west-1:123456789012:task/prod/task1"
	api := &fakeECS{
		clusterPages: [][]string{{"arn:aws:ecs:eu-west-1:123456789012:cluster/prod"}},
		taskPages:    map[string][][]string{"prod": {{arn}}},
		tasks: map[string]ecstypes.Task{
			arn: runningTask(arn, "service:worker", false,
				execContainer("app", "", ""),
				execContainer("logs", "", "")),
		},
	}

	got, err := ListECS(context.Background(), api, "", "app")
	if err != nil {
		t.Fatalf("ListECS: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("container filter %q matched %d targets, want 1: %+v", "app", len(got), got)
	}
	ok, why := got[0].Ready()
	if ok {
		t.Error("a task without enableExecuteCommand must not be reported ready")
	}
	if why == "" {
		t.Error("Ready() gave no reason for an exec-disabled task")
	}
}
