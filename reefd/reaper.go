package reefd

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type reaper struct {
	ec2 ec2Client
}

func newReaper(ec2 ec2Client) *reaper {
	return &reaper{ec2: ec2}
}

func (r *reaper) listDeadWindowsInstances(ctx context.Context) ([]string, error) {
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
	input := &ec2.DescribeInstancesInput{Filters: filters}
	result, err := r.ec2.DescribeInstances(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	const instanceAgeLimit = -4 * time.Hour

	cut := time.Now().Add(instanceAgeLimit)

	var instances []string
	for _, r := range result.Reservations {
		for _, instance := range r.Instances {
			if instance.LaunchTime.Before(cut) {
				instances = append(instances, *instance.InstanceId)
			}
		}
	}

	return instances, nil
}

func (r *reaper) reapDeadWindowsInstances(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		log.Print("no dead windows instances to reap")
		return nil
	}

	log.Printf("reaping %d dead windows instances: %v", len(ids), ids)
	input := &ec2.TerminateInstancesInput{InstanceIds: ids}
	_, err := r.ec2.TerminateInstances(ctx, input)
	if err == nil {
		log.Printf("terminated %d dead windows instances: %v", len(ids), ids)
	}
	return err
}

func (r *reaper) listAndReapDeadWindowsInstances(ctx context.Context) error {
	instances, err := r.listDeadWindowsInstances(ctx)
	if err != nil {
		return err
	}
	return r.reapDeadWindowsInstances(ctx, instances)
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
	return r.listAndReapDeadWindowsInstances(ctx)
}
