package target

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type SSMLister interface {
	DescribeInstanceInformation(context.Context, *ssm.DescribeInstanceInformationInput, ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
}

type EC2Describer interface {
	DescribeInstances(context.Context, *ec2.DescribeInstancesInput, ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func ListEC2(ctx context.Context, ssmc SSMLister, ec2c EC2Describer, filter string) ([]Target, error) {
	var items []ssmtypes.InstanceInformation
	var token *string
	for {
		out, err := ssmc.DescribeInstanceInformation(ctx, &ssm.DescribeInstanceInformationInput{NextToken: token})
		if err != nil {
			return nil, fmt.Errorf("describe instance information: %w", err)
		}
		items = append(items, out.InstanceInformationList...)
		if out.NextToken == nil {
			break
		}
		token = out.NextToken
	}

	names := ec2Names(ctx, ec2c, items)

	var out []Target
	for _, it := range items {
		id := aws.ToString(it.InstanceId)
		if id == "" {
			continue
		}
		t := Target{
			Kind:         KindEC2,
			InstanceID:   id,
			Name:         names[id],
			Platform:     string(it.PlatformType),
			AgentVersion: aws.ToString(it.AgentVersion),
			PingStatus:   string(it.PingStatus),
		}
		if t.Matches(filter) {
			out = append(out, t)
		}
	}
	return out, nil
}

func ec2Names(ctx context.Context, ec2c EC2Describer, items []ssmtypes.InstanceInformation) map[string]string {
	names := make(map[string]string, len(items))
	if ec2c == nil || len(items) == 0 {
		return names
	}

	ids := make([]string, 0, len(items))
	for _, it := range items {
		if it.InstanceId != nil {
			ids = append(ids, *it.InstanceId)
		}
	}

	const chunk = 100
	for start := 0; start < len(ids); start += chunk {
		end := min(start+chunk, len(ids))
		out, err := ec2c.DescribeInstances(ctx, &ec2.DescribeInstancesInput{InstanceIds: ids[start:end]})
		if err != nil {
			return names
		}
		for _, r := range out.Reservations {
			for _, inst := range r.Instances {
				names[aws.ToString(inst.InstanceId)] = nameTag(inst.Tags)
			}
		}
	}
	return names
}

func nameTag(tags []ec2types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
