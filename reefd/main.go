package reefd

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	_ "github.com/mattn/go-sqlite3"
)

type InstanceInfo struct {
	InstanceType string `json:"instance_type"`
	AMI          string `json:"ami"`
	State        string `json:"state"`
}

type LaunchRequest struct {
	Id           string        `json:"id"`
	DesiredState InstanceInfo  `json:"desired_state"`
	CurrentState *InstanceInfo `json:"current_state"`
	InstanceId   *string       `json:"instance_id,omitempty"`
}

const (
	defaultRegion = "us-west-2"
)

type instanceManager struct {
	db         *sql.DB
	ec2Client  EC2Client
}

// launchInstance launches an instance with the given instance type and AMI
func (m *instanceManager) launchInstance(instanceType, ami string) (string, error) {
	svc := m.ec2Client
	if svc == nil {
		return "", fmt.Errorf("failed to get EC2 client")
	}
	log.Printf("Launching instance with type: %s, AMI: %s", instanceType, ami)

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
func (m *instanceManager) getInstanceState(instanceID string) (*InstanceInfo, error) {
	log.Printf("Getting instance state for %s", instanceID)
	svc := m.ec2Client
	if svc == nil {
		return nil, fmt.Errorf("ec2 client does not exist")
	}

	result, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	})
	if err != nil {
		return nil, fmt.Errorf("error describing instance %s: %v", instanceID, err)
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("no instance found with ID %s", instanceID)
	}

	instance := result.Reservations[0].Instances[0]
	return &InstanceInfo{
		InstanceType: *instance.InstanceType,
		AMI:          *instance.ImageId,
		State:        *instance.State.Name,
	}, nil
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

		if state != nil {
			if currentStateJSON, err := json.Marshal(state); err == nil {
				currentStateMap[id] = string(currentStateJSON)
			}
		}
	}

	for id, currentStateJSON := range currentStateMap {
		if _, err := m.db.Exec(`UPDATE launch_requests SET current_state = ? WHERE id = ?`, currentStateJSON, id); err != nil {
			return fmt.Errorf("Update current state for each request: %v", err)
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

func main() {
	dbPath := flag.String("db", "", "Path to .db file")
	flag.Parse()

	if *dbPath == "" {
		log.Fatal("Database path is required")
	}

	if _, err := os.Stat(*dbPath); err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("File %s does not exist", *dbPath)
		}
		log.Fatalf("Error checking database file %s: %v", *dbPath, err)
	}

	db, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		log.Fatalf("Error connecting to database: %s", err)
	}
	defer db.Close()

	ec2Client := getEC2Client()

	http.HandleFunc("/launch", func(w http.ResponseWriter, r *http.Request) {
		handleLaunchRequest(db, w, r, ec2Client)
	})

	log.Printf("Starting server on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleLaunchRequest retrieves the desired state from the request body and inserts it into the database
// then starts a goroutine to process the launch requests
func handleLaunchRequest(db *sql.DB, w http.ResponseWriter, r *http.Request, ec2Client EC2Client) {
	var desiredState InstanceInfo
	if err := json.NewDecoder(r.Body).Decode(&desiredState); err != nil {
		http.Error(w, "Invalid JSON format: "+err.Error(), http.StatusBadRequest)
		return
	}
	// marshal the desired state to a json string
	desiredJSON, err := json.Marshal(desiredState)
	if err != nil {
		http.Error(w, "Error marshaling desired state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// insert the desired state into the database
	if _, err := db.Exec(`INSERT INTO launch_requests (desired_state) VALUES (?)`, string(desiredJSON)); err != nil {
		http.Error(w, "Error inserting into database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// start a goroutine to scan the database for launch requests with different desired and current states
	instanceManager := &instanceManager{db: db, ec2Client: ec2Client}
	go processLaunchRequests(instanceManager)
}

// processLaunchRequests updates the current state of the existing instances and launches new instances for the launch requests that have not been launched yet
func processLaunchRequests(instanceManager *instanceManager) error {
	// update the current state of the existing instances
	if err := instanceManager.updateCurrentState(); err != nil {
		return fmt.Errorf("error updating current state: %v", err)
	}

	// query all launch requests where the instance has not been launched yet
	log.Printf("Scanning for launch requests with different desired and current states")
	rows, err := instanceManager.db.Query("SELECT id, desired_state FROM launch_requests WHERE current_state IS NULL OR instance_id IS NULL")
	if err != nil {
		return fmt.Errorf("error querying database: %v", err)
	}
	defer rows.Close()

	instanceIDMap := make(map[string]string)

	// iterate over all matching launch requests
	for rows.Next() {
		var id, desiredStateJSON string
		if err := rows.Scan(&id, &desiredStateJSON); err != nil {
			log.Printf("Error scanning row: %s", err)
			continue
		}

		var desiredState InstanceInfo
		if err := json.Unmarshal([]byte(desiredStateJSON), &desiredState); err != nil {
			log.Printf("Error unmarshalling desired state: %s", err)
			continue
		}

		// launch instance
		instanceID, err := instanceManager.launchInstance(desiredState.InstanceType, desiredState.AMI)
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
