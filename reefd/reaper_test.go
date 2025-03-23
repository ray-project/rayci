package reefd

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type fakeEC2Instance struct {
	stateCode string
	tags      map[string]string
}

func (i *fakeEC2Instance) name() string {
	return i.tags["Name"]
}

type fakeEC2 struct {
	instances []*fakeEC2Instance
}

func (c *fakeEC2) DescribeInstances(
	ctx context.Context, input *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	output := &ec2.DescribeInstancesOutput{}
	for _, instance := range c.instances {
		for _, filter := range input.Filters {
			_ = filter
		}

		_ = instance
	}

	return output, nil
}
