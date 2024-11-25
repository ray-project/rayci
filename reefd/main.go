package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	_ "github.com/mattn/go-sqlite3"
)

type instanceInfo struct {
	InstanceType string `json:"instance_type"`
	AMI          string `json:"ami"`
	State        string `json:"state"`
}

type launchRequest struct {
	Id           string        `json:"id"`
	DesiredState instanceInfo  `json:"desired_state"`
	CurrentState *instanceInfo `json:"current_state"`
	InstanceId   *string       `json:"instance_id,omitempty"`
}

const (
	region = "us-west-2"
)

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

	http.HandleFunc("/launch", func(w http.ResponseWriter, r *http.Request) {
		handleLaunchRequest(db, w, r)
	})

	log.Printf("Starting server on port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// handleLaunchRequest retrieves the desired state from the request body and inserts it into the database
// then starts a goroutine to process the launch requests
func handleLaunchRequest(db *sql.DB, w http.ResponseWriter, r *http.Request) {
	var desiredState instanceInfo
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
	go processLaunchRequests(db)
}

// launchInstance launches an instance with the given instance type and AMI
func launchInstance(instanceType, ami string) string {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		log.Printf("Failed to create session: %v", err)
		return ""
	}

	svc := ec2.New(sess)
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
		log.Printf("Failed to launch instance: %v", err)
		return ""
	}
	instanceID := *runResult.Instances[0].InstanceId
	log.Printf("Created instance: %s", instanceID)
	return instanceID
}

// getInstanceState retrieves the current state of the instance from AWS with the given instance ID
func getInstanceState(instanceID string) *instanceInfo {
	sess, err := session.NewSession(&aws.Config{Region: aws.String(region)})
	if err != nil {
		log.Printf("Error creating session: %s", err)
		return nil
	}

	svc := ec2.New(sess)
	result, err := svc.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{aws.String(instanceID)},
	})
	if err != nil {
		log.Printf("Error describing instance %s: %s", instanceID, err)
		return nil
	}

	if len(result.Reservations) == 0 || len(result.Reservations[0].Instances) == 0 {
		return nil
	}

	instance := result.Reservations[0].Instances[0]
	return &instanceInfo{
		InstanceType: *instance.InstanceType,
		AMI:          *instance.ImageId,
		State:        *instance.State.Name,
	}
}

// updateCurrentState updates the current state of the existing instances in the database table
func updateCurrentState(db *sql.DB) {
	rows, err := db.Query("SELECT id, instance_id FROM launch_requests WHERE instance_id IS NOT NULL")
	if err != nil {
		log.Printf("Error querying database: %s", err)
		return
	}
	defer rows.Close()

	currentStateMap := make(map[string]string)
	for rows.Next() {
		var id, instanceID string
		if err := rows.Scan(&id, &instanceID); err != nil {
			log.Printf("Error scanning row: %s", err)
			continue
		}

		if state := getInstanceState(instanceID); state != nil {
			if currentStateJSON, err := json.Marshal(state); err == nil {
				currentStateMap[id] = string(currentStateJSON)
			}
		}
	}

	for id, currentStateJSON := range currentStateMap {
		log.Printf("Updating current state for request %s: %s", id, currentStateJSON)
		if _, err := db.Exec(`UPDATE launch_requests SET current_state = ? WHERE id = ?`, currentStateJSON, id); err != nil {
			log.Printf("Error updating current state for request %s: %s", id, err)
		}
	}
}

// processLaunchRequests updates the current state of the existing instances and launches new instances for the launch requests that have not been launched yet
func processLaunchRequests(db *sql.DB) {
	// update the current state of the existing instances
	updateCurrentState(db)

	// query all launch requests where the instance has not been launched yet
	log.Printf("Scanning for launch requests with different desired and current states")
	rows, err := db.Query("SELECT id, desired_state FROM launch_requests WHERE current_state IS NULL OR instance_id IS NULL")
	if err != nil {
		log.Printf("Error querying database: %s", err)
		return
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

		var desiredState instanceInfo
		if err := json.Unmarshal([]byte(desiredStateJSON), &desiredState); err != nil {
			log.Printf("Error unmarshalling desired state: %s", err)
			continue
		}

		// launch instance
		if instanceID := launchInstance(desiredState.InstanceType, desiredState.AMI); instanceID != "" {
			instanceIDMap[id] = instanceID // map the id to the instance id to update on the db later
		}
	}

	// update the launch requests with the instance id
	for id, instanceID := range instanceIDMap {
		log.Printf("Updating instance ID for request %s: %s", id, instanceID)
		if _, err := db.Exec(`UPDATE launch_requests SET instance_id = ? WHERE id = ?`, instanceID, id); err != nil {
			log.Printf("Error updating instance ID for request %s: %s", id, err)
		}
	}
}
