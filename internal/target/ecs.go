package target

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

type ECSAPI interface {
	ListClusters(context.Context, *ecs.ListClustersInput, ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListTasks(context.Context, *ecs.ListTasksInput, ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(context.Context, *ecs.DescribeTasksInput, ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

func ListECS(ctx context.Context, api ECSAPI, cluster, filter string) ([]Target, error) {
	clusters, err := listClusters(ctx, api, cluster)
	if err != nil {
		return nil, err
	}

	var out []Target
	for _, c := range clusters {
		tasks, err := describeRunningTasks(ctx, api, c)
		if err != nil {
			continue
		}
		for _, task := range tasks {
			out = append(out, containersOf(c, task, filter)...)
		}
	}
	return out, nil
}

func listClusters(ctx context.Context, api ECSAPI, filter string) ([]string, error) {
	var names []string
	var token *string
	for {
		out, err := api.ListClusters(ctx, &ecs.ListClustersInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("list clusters: %w", err)
		}
		for _, arn := range out.ClusterArns {
			name := arnTail(arn)
			if filter == "" || strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
				names = append(names, name)
			}
		}
		if out.NextToken == nil {
			break
		}
		token = out.NextToken
	}
	return names, nil
}

func describeRunningTasks(ctx context.Context, api ECSAPI, cluster string) ([]ecstypes.Task, error) {
	var arns []string
	var token *string
	for {
		out, err := api.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       aws.String(cluster),
			DesiredStatus: ecstypes.DesiredStatusRunning,
			NextToken:     token,
		})
		if err != nil {
			return nil, fmt.Errorf("list tasks in %s: %w", cluster, err)
		}
		arns = append(arns, out.TaskArns...)
		if out.NextToken == nil {
			break
		}
		token = out.NextToken
	}
	if len(arns) == 0 {
		return nil, nil
	}

	const chunk = 100
	var tasks []ecstypes.Task
	for start := 0; start < len(arns); start += chunk {
		end := min(start+chunk, len(arns))
		out, err := api.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   arns[start:end],
		})
		if err != nil {
			return nil, fmt.Errorf("describe tasks in %s: %w", cluster, err)
		}
		tasks = append(tasks, out.Tasks...)
	}
	return tasks, nil
}

func containersOf(cluster string, task ecstypes.Task, filter string) []Target {
	taskID := arnTail(aws.ToString(task.TaskArn))
	service := serviceFromGroup(aws.ToString(task.Group))
	status := aws.ToString(task.LastStatus)

	var out []Target
	for _, c := range task.Containers {
		t := Target{
			Kind:        KindECS,
			Cluster:     cluster,
			Service:     service,
			TaskID:      taskID,
			TaskARN:     aws.ToString(task.TaskArn),
			Container:   aws.ToString(c.Name),
			RuntimeID:   aws.ToString(c.RuntimeId),
			TaskStatus:  status,
			ExecEnabled: task.EnableExecuteCommand,
			ExecAgent:   execAgentStatus(c.ManagedAgents),
		}
		if t.Matches(filter) {
			out = append(out, t)
		}
	}
	return out
}

func execAgentStatus(agents []ecstypes.ManagedAgent) string {
	for _, a := range agents {
		if a.Name == ecstypes.ManagedAgentNameExecuteCommandAgent {
			return aws.ToString(a.LastStatus)
		}
	}
	return ""
}

func serviceFromGroup(group string) string {
	if rest, ok := strings.CutPrefix(group, "service:"); ok {
		return rest
	}
	return group
}

func arnTail(arn string) string {
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}
