package reefd

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type InstanceConfiguration struct {
	InstanceType string `json:"instance_type"`
	AMI          string `json:"ami"`
}

// LaunchRequest represents a row of launch_requests table in the database
type LaunchRequest struct {
	Id           string `json:"id"`
	InstanceConfigName string `json:"instance_config_name"`
	DesiredState string `json:"desired_state"`
	CurrentState string `json:"current_state"`
	InstanceId   *string `json:"instance_id,omitempty"`
}

const (
	defaultRegion = "us-west-2"
)

type instanceManager struct {
	db        *sql.DB
	ec2Client EC2Client
}

func (m *instanceManager) getInstanceConfig(instanceConfigName string) (InstanceConfiguration, error) {
	var instanceConfig InstanceConfiguration
	err := m.db.QueryRow("SELECT instance_type, ami FROM instance_configs WHERE name = ?", instanceConfigName).Scan(&instanceConfig.InstanceType, &instanceConfig.AMI)
	if err != nil {
		return InstanceConfiguration{}, fmt.Errorf("error querying database: %v", err)
	}
	return instanceConfig, nil
}

// launchInstance launches an instance with the given instance type and AMI
func (m *instanceManager) launchInstance(instanceConfigName string) (string, error) {
	svc := m.ec2Client
	if svc == nil {
		return "", fmt.Errorf("failed to get EC2 client")
	}
	log.Printf("Launching instance with config: %s", instanceConfigName)
	instanceConfig, err := m.getInstanceConfig(instanceConfigName)
	if err != nil {
		return "", fmt.Errorf("error getting instance config: %v", err)
	}
	instanceType := instanceConfig.InstanceType
	ami := instanceConfig.AMI

	runResult, err := svc.RunInstances(&ec2.RunInstancesInput{
		ImageId:      aws.String(ami),
		InstanceType: aws.String(instanceType),
		MinCount:     aws.Int64(1),
		MaxCount:     aws.Int64(1),
		TagSpecifications: []*ec2.TagSpecification{{
			ResourceType: aws.String("instance"),
			Tags: []*ec2.Tag{{
				Key:   aws.String("Name"),
				Value: aws.String("Kevin-launch"),
			}},
		}},
	})

	if err != nil {
		return "", fmt.Errorf("failed to launch instance: %v", err)
	}
	instanceID := *runResult.Instances[0].InstanceId
	log.Printf("Created instance: %s", instanceID)
	return instanceID, nil
}

// getInstanceState retrieves the current state of the instance from AWS with the given instance ID
func (m *instanceManager) getInstanceState(instanceID string) (string, error) {
	log.Printf("Getting instance state for %s", instanceID)
	svc := m.ec2Client
	if svc == nil {
		return "", fmt.Errorf("ec2 client does not exist")
	}

	result, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	})
	if err != nil {
		return "", fmt.Errorf("error describing instance %s: %v", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return "", fmt.Errorf("no instance found with ID %s", instanceID)
	}

	instance := result.Reservations[0].Instances[0]
	return *instance.State.Name, nil
}


// updateCurrentState updates the current state of the existing instances in the database table
func (m *instanceManager) updateCurrentState() error {
	log.Printf("Updating current state of existing instances")
	rows, err := m.db.Query("SELECT id, instance_id FROM launch_requests WHERE instance_id IS NOT NULL")
	if err != nil {
		return fmt.Errorf("error querying database: %v", err)
	}
	defer rows.Close()

	currentStateMap := make(map[string]string)
	for rows.Next() {
		var id, instanceID string
		if err := rows.Scan(&id, &instanceID); err != nil {
			log.Printf("Error scanning row: %s", err)
			continue
		}

		state, err := m.getInstanceState(instanceID)
		if err != nil {
			log.Printf("Error getting instance state: %v", err)
			continue
		}

		currentStateMap[id] = state
	}

	for id, currentState := range currentStateMap {
		if _, err := m.db.Exec(`UPDATE launch_requests SET current_state = ? WHERE id = ?`, currentState, id); err != nil {
			return fmt.Errorf("Error updating current state for each request: %v", err)
		}
	}
	return nil
}

type EC2Client interface {
	RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error)
	DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
}

func getEC2Client() EC2Client {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(defaultRegion)})
	if err != nil {
		log.Fatalf("Error creating session: %v", err)
	}
	return ec2.New(sess)
}

// processLaunchRequests updates the current state of the existing instances and launches new instances for the launch requests that have not been launched yet
func processLaunchRequests(instanceManager *instanceManager) error {
	// update the current state of the existing instances
	if err := instanceManager.updateCurrentState(); err != nil {
		return fmt.Errorf("error updating current state: %v", err)
	}

	// query all launch requests where the instance has not been launched yet
	log.Printf("Scanning for launch requests with different desired and current states")
	rows, err := instanceManager.db.Query("SELECT id, instance_config_name FROM launch_requests WHERE current_state IS NULL OR instance_id IS NULL")
	if err != nil {
		return fmt.Errorf("error querying database: %v", err)
	}
	defer rows.Close()

	instanceIDMap := make(map[string]string)

	// iterate over all matching launch requests
	for rows.Next() {
		var id, instanceConfigName string
		if err := rows.Scan(&id, &instanceConfigName); err != nil {
			log.Printf("Error scanning row: %s", err)
			continue
		}

		// launch instance
		instanceID, err := instanceManager.launchInstance(instanceConfigName)
		if err != nil {
			log.Printf("Error launching instance: %v", err)
			continue
		}
		if instanceID != "" {
			instanceIDMap[id] = instanceID // map the id to the instance id to update on the db later
		}
	}

	// update the launch requests with the instance id
	for id, instanceID := range instanceIDMap {
		log.Printf("Updating instance ID for request %s: %s", id, instanceID)
		if _, err := instanceManager.db.Exec(`UPDATE launch_requests SET instance_id = ? WHERE id = ?`, instanceID, id); err != nil {
			return fmt.Errorf("error updating instance ID for request %s: %v", id, err)
		}
	}
	return nil
}
