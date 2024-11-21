package main

import (
	"fmt"
)

type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

func listInstances(api EC2DescribeInstancesAPI) {
	filters := []types.Filter{
		{
			Name: aws.String("tag:reefd"),
			Values: []string{"true"},
		},
	}
	input := &ec2.DescribeInstancesInput{
		Filters: filters,
	}
	result, err := api.DescribeInstances(context.Background(), input)
	if err != nil {
		log.Fatalf("failed to describe instances: %v", err)
	}
	fmt.Println(result)

	// make a counter, key is machine config name, value is count of instances
	instanceCounter := make(map[string]int)
	for _, reservation := range result.Reservations {
		for _, instance := range reservation.Instances {
			instanceCounter[*instance.Tags[0].Value]++
		}
	}
	fmt.Println(instanceCounter)
	for machineConfig, count := range instanceCounter {
		fmt.Printf("%s: %d\n", machineConfig, count)
	}
}

func main() {
	// List all EC2 instances with tag reefd
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	client := ec2.NewFromConfig(cfg)
	listInstances(client)
}
