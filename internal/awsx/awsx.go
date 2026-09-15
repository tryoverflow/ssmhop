package awsx

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

type Clients struct {
	Profile string
	Region  string

	SSM *ssm.Client
	EC2 *ec2.Client
	ECS *ecs.Client
}

func Load(ctx context.Context, profile, region string) (*Clients, error) {
	var opts []func(*config.LoadOptions) error
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	if cfg.Region == "" {
		return nil, errors.New("no region resolved: pass --region, or set AWS_REGION / a region in your profile")
	}

	return &Clients{
		Profile: profile,
		Region:  cfg.Region,
		SSM:     ssm.NewFromConfig(cfg),
		EC2:     ec2.NewFromConfig(cfg),
		ECS:     ecs.NewFromConfig(cfg),
	}, nil
}
