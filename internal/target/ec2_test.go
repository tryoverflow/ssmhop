package target

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type fakeSSM struct {
	pages [][]ssmtypes.InstanceInformation
	fail  bool
}

func (f *fakeSSM) DescribeInstanceInformation(_ context.Context, in *ssm.DescribeInstanceInformationInput, _ ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error) {
	if f.fail {
		return nil, errors.New("not authorized to perform ssm:DescribeInstanceInformation")
	}
	page := 0
	if in.NextToken != nil {
		fmt.Sscanf(*in.NextToken, "%d", &page)
	}
	out := &ssm.DescribeInstanceInformationOutput{InstanceInformationList: f.pages[page]}
	if page+1 < len(f.pages) {
		out.NextToken = aws.String(fmt.Sprint(page + 1))
	}
	return out, nil
}

type fakeEC2 struct {
	names      map[string]string
	fail       bool
	batchSizes []int
}

func (f *fakeEC2) DescribeInstances(_ context.Context, in *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	f.batchSizes = append(f.batchSizes, len(in.InstanceIds))
	if f.fail {
		return nil, errors.New("not authorized to perform ec2:DescribeInstances")
	}
	var insts []ec2types.Instance
	for _, id := range in.InstanceIds {
		inst := ec2types.Instance{InstanceId: aws.String(id)}
		if name, ok := f.names[id]; ok {
			inst.Tags = []ec2types.Tag{
				{Key: aws.String("env"), Value: aws.String("prod")},
				{Key: aws.String("Name"), Value: aws.String(name)},
			}
		}
		insts = append(insts, inst)
	}
	return &ec2.DescribeInstancesOutput{
		Reservations: []ec2types.Reservation{{Instances: insts}},
	}, nil
}

func instanceInfo(id, ping string) ssmtypes.InstanceInformation {
	return ssmtypes.InstanceInformation{
		InstanceId:   aws.String(id),
		PingStatus:   ssmtypes.PingStatus(ping),
		PlatformType: ssmtypes.PlatformTypeLinux,
		AgentVersion: aws.String("3.2.1377.0"),
	}
}

func TestListEC2(t *testing.T) {
	ssmc := &fakeSSM{pages: [][]ssmtypes.InstanceInformation{
		{instanceInfo("i-aaa", "Online")},
		{instanceInfo("i-bbb", "ConnectionLost")},
	}}
	ec2c := &fakeEC2{names: map[string]string{"i-aaa": "web-1", "i-bbb": "db-1"}}

	got, err := ListEC2(context.Background(), ssmc, ec2c, "")
	if err != nil {
		t.Fatalf("ListEC2: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d targets across 2 pages, want 2: %+v", len(got), got)
	}
	if got[0].InstanceID != "i-aaa" || got[0].Name != "web-1" {
		t.Errorf("Name tag not resolved: %+v", got[0])
	}
	if got[0].SSMTarget() != "i-aaa" {
		t.Errorf("SSMTarget() = %q, want the instance ID", got[0].SSMTarget())
	}
	if ok, _ := got[0].Ready(); !ok {
		t.Error("an Online instance should be ready")
	}
	if ok, why := got[1].Ready(); ok || why == "" {
		t.Error("a ConnectionLost instance should be not-ready with a reason")
	}
}

func TestListEC2ToleratesTagFailure(t *testing.T) {
	ssmc := &fakeSSM{pages: [][]ssmtypes.InstanceInformation{{instanceInfo("i-aaa", "Online")}}}
	ec2c := &fakeEC2{fail: true}

	got, err := ListEC2(context.Background(), ssmc, ec2c, "")
	if err != nil {
		t.Fatalf("ListEC2 should tolerate a DescribeInstances failure, got %v", err)
	}
	if len(got) != 1 || got[0].InstanceID != "i-aaa" || got[0].Name != "" {
		t.Fatalf("want one unnamed instance, got %+v", got)
	}
}

func TestListEC2WithoutEC2Client(t *testing.T) {
	ssmc := &fakeSSM{pages: [][]ssmtypes.InstanceInformation{{instanceInfo("i-aaa", "Online")}}}
	got, err := ListEC2(context.Background(), ssmc, nil, "")
	if err != nil {
		t.Fatalf("ListEC2 with a nil EC2 client: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v, want one target", got)
	}
}

func TestListEC2ChunksDescribeInstances(t *testing.T) {
	const total = 150
	page := make([]ssmtypes.InstanceInformation, 0, total)
	for i := range total {
		page = append(page, instanceInfo(fmt.Sprintf("i-%03d", i), "Online"))
	}
	ssmc := &fakeSSM{pages: [][]ssmtypes.InstanceInformation{page}}
	ec2c := &fakeEC2{names: map[string]string{}}

	got, err := ListEC2(context.Background(), ssmc, ec2c, "")
	if err != nil {
		t.Fatalf("ListEC2: %v", err)
	}
	if len(got) != total {
		t.Errorf("got %d targets, want %d", len(got), total)
	}
	if len(ec2c.batchSizes) != 2 {
		t.Fatalf("DescribeInstances batches %v, want 2", ec2c.batchSizes)
	}
	for _, n := range ec2c.batchSizes {
		if n > 100 {
			t.Errorf("DescribeInstances batch of %d exceeds 100", n)
		}
	}
}

func TestListEC2Filter(t *testing.T) {
	ssmc := &fakeSSM{pages: [][]ssmtypes.InstanceInformation{{
		instanceInfo("i-aaa", "Online"),
		instanceInfo("i-bbb", "Online"),
	}}}
	ec2c := &fakeEC2{names: map[string]string{"i-aaa": "web-prod", "i-bbb": "db-prod"}}

	got, err := ListEC2(context.Background(), ssmc, ec2c, "web")
	if err != nil {
		t.Fatalf("ListEC2: %v", err)
	}
	if len(got) != 1 || got[0].Name != "web-prod" {
		t.Fatalf("filter %q should match one instance by Name tag, got %+v", "web", got)
	}
}

func TestListEC2PropagatesFailure(t *testing.T) {
	if _, err := ListEC2(context.Background(), &fakeSSM{fail: true}, nil, ""); err == nil {
		t.Fatal("ListEC2 should surface a DescribeInstanceInformation failure")
	}
}
