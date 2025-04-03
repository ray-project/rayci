package reefd

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type reaper struct {
	ec2     ec2Client
	nowFunc func() time.Time
}

func newReaper(ec2 ec2Client) *reaper {
	return &reaper{ec2: ec2}
}

func (r *reaper) now() time.Time {
	if r.nowFunc != nil {
		return r.nowFunc()
	}
	return time.Now()
}

func (r *reaper) setNowFunc(f func() time.Time) {
	r.nowFunc = f
}

func (r *reaper) listDeadWindowsInstances(ctx context.Context) (
	[]string, error,
) {
	filters := []types.Filter{{
		Name:   aws.String("tag:BuildkiteQueue"),
		Values: []string{"*windows*"},
	}, {
		Name: aws.String("instance-state-code"),
		Values: []string{
			"0",  // pending
			"16", // running
		},
	}}
	const maxResults = 500
	input := &ec2.DescribeInstancesInput{
		Filters:    filters,
		MaxResults: aws.Int32(maxResults),
	}
	result, err := r.ec2.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	const instanceAgeLimit = -4 * time.Hour

	cut := r.now().Add(instanceAgeLimit)

	var instances []string
	for _, r := range result.Reservations {
		for _, i := range r.Instances {
			if i.LaunchTime.Before(cut) {
				instances = append(instances, *i.InstanceId)
			}
		}
	}

	sort.Strings(instances)

	return instances, nil
}

func (r *reaper) terminateInstances(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	input := &ec2.TerminateInstancesInput{InstanceIds: ids}
	_, err := r.ec2.TerminateInstances(ctx, input)
	return err
}

func (r *reaper) listAndReapDeadWindowsInstances(ctx context.Context) (
	int, error,
) {
	ids, err := r.listDeadWindowsInstances(ctx)
	if err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}

	log.Printf("terminating %d instances: %v", len(ids), ids)
	if err := r.terminateInstances(ctx, ids); err != nil {
		return 0, err
	}

	return len(ids), nil
}

// ReapDeadWindowsInstances lists and terminates dead Windows CI instances.
func ReapDeadWindowsInstances(ctx context.Context) error {
	awsConfig, err := awsconfig.LoadDefaultConfig(
		ctx, awsconfig.WithRegion(awsRegion),
	)
	if err != nil {
		return fmt.Errorf("load aws config: %w", err)
	}

	clients := newAWSClientsFromConfig(&awsConfig)
	r := newReaper(clients.ec2())
	if _, err := r.listAndReapDeadWindowsInstances(ctx); err != nil {
		return err
	}
	return nil
}
