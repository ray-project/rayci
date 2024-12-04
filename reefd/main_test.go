package reefd

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)


type mockEC2Client struct {
	describeInstancesOutput *ec2.DescribeInstancesOutput
	runInstancesOutput *ec2.Reservation
	err error
}

func (m *mockEC2Client) DescribeInstances(*ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return m.describeInstancesOutput, m.err
}

func (m *mockEC2Client) RunInstances(*ec2.RunInstancesInput) (*ec2.Reservation, error) {
	return m.runInstancesOutput, m.err
}

const sampleDesiredState = `{"instance_type":"t3.micro","ami":"ami-1234567890abcdef0","state":"running"}`

func setupTestDB(t *testing.T) (*sql.DB, string) {
	dbPath := filepath.Join(os.TempDir(), "test_db.sqlite3")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Error opening test database: %s", err)
	}

	// create a test launch_requests table
	_, err = db.Exec(`CREATE TABLE launch_requests (id TEXT PRIMARY KEY, instance_id TEXT, desired_state TEXT, current_state TEXT)`)
	if err != nil {
		t.Fatalf("Error creating test launch_requests table: %s", err)
	}

	return db, dbPath
}

func TestGetInstanceState(t *testing.T) {
	ec2Svc := &mockEC2Client{
		describeInstancesOutput: &ec2.DescribeInstancesOutput{
			Reservations: []*ec2.Reservation{{
				Instances: []*ec2.Instance{{
					InstanceType: aws.String("t3.micro"),
					ImageId:      aws.String("ami-1234567890abcdef0"),
					State: &ec2.InstanceState{
						Name: aws.String("running"),
					},
				}},
			}},
		},
		err: nil,
	}
	getEC2Client = func() EC2Client {
		return ec2Svc
	}
	instanceInfo := getInstanceState("i-1234567890abcdef0")
	want := &InstanceInfo{
		InstanceType: "t3.micro",
		AMI:          "ami-1234567890abcdef0",
		State:        "running",
	}
	if !reflect.DeepEqual(instanceInfo, want) {
		t.Errorf("got %v, want %v", instanceInfo, want)
	}
}

func TestLaunchInstance(t *testing.T) {
	ec2Svc := &mockEC2Client{
		runInstancesOutput: &ec2.Reservation{
			Instances: []*ec2.Instance{{
				InstanceId: aws.String("i-1234567890abcdef0"),
			}},
		},
	}
	getEC2Client = func() EC2Client {
		return ec2Svc
	}
	instanceID := launchInstance("t3.micro", "ami-1234567890abcdef0")
	want := "i-1234567890abcdef0"
	if instanceID != want {
		t.Errorf("got %q, want %q", instanceID, want)
	}
}

func TestUpdateCurrentState(t *testing.T) {
	getInstanceState = func(instanceID string) *InstanceInfo {
		return &InstanceInfo{
			InstanceType: "t3.micro",
			AMI:          "ami-1234567890abcdef0",
			State:        "running",
		}
	}

	db, dbPath := setupTestDB(t)
	defer db.Close()
	defer os.Remove(dbPath)

	// insert a test launch_requests row without current_state set
	_, err := db.Exec(`INSERT INTO launch_requests (id, instance_id, desired_state,	 current_state) VALUES (?, ?, ?, ?)`, "1", "i-1234567890abcdef0", sampleDesiredState, nil)
	if err != nil {
		t.Fatalf("Error inserting test launch_requests row: %s", err)
	}

	updateCurrentState(db)

	// check that the current_state column has been updated
	var currentState string
	err = db.QueryRow(`SELECT current_state FROM launch_requests WHERE id = ?`, "1").Scan(&currentState)
	if err != nil {
		t.Fatalf("Error querying current_state column: %s", err)
	}

	want := sampleDesiredState
	if currentState != want {
		t.Errorf("got %q, want %q", currentState, want)
	}
}

func TestProcessLaunchRequests(t *testing.T) {
	updateCurrentState = func(db *sql.DB) {
		// do nothing
	}
	launchInstance = func(instanceType, ami string) string {
		return "i-1234567890abcdef1"
	}

	db, dbPath := setupTestDB(t)
	defer db.Close()
	defer os.Remove(dbPath)

	// insert a test launch_requests row without current_state set
	_, err := db.Exec(`INSERT INTO launch_requests (id, instance_id, desired_state, current_state) VALUES (?, ?, ?, ?)`, "1", "i-1234567890abcdef0", sampleDesiredState, nil)
	if err != nil {
		t.Fatalf("Error inserting test launch_requests row: %s", err)
	}

	processLaunchRequests(db)

	// check that the instance_id column has been updated correctly
	var instanceID string
	err = db.QueryRow(`SELECT instance_id FROM launch_requests WHERE id = ?`, "1").Scan(&instanceID)
	if err != nil {
		t.Fatalf("Error querying instance_id column: %s", err)
	}

	want := "i-1234567890abcdef1"
	if instanceID != want {
		t.Errorf("got %q, want %q", instanceID, want)
	}
}
