package reefd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type ec2Client interface {
	DescribeInstances(
		ctx context.Context,
		params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.DescribeInstancesOutput, error)

	TerminateInstances(
		ctx context.Context,
		params *ec2.TerminateInstancesInput,
		optFns ...func(*ec2.Options),
	) (*ec2.TerminateInstancesOutput, error)
}

type awsClients struct {
	ec2 func() ec2Client
}

func newAWSClientsFromConfig(cfg *aws.Config) *awsClients {
	return &awsClients{
		ec2: func() ec2Client { return ec2.NewFromConfig(*cfg) },
	}
}

const awsRegion = "us-west-2"

func newAWSClients(ctx context.Context) (*awsClients, error) {
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx, awsconfig.WithRegion(awsRegion),
	)
	if err != nil {
		return nil, err
	}
	return newAWSClientsFromConfig(&cfg), nil
}
