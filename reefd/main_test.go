package reefd

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type mockEC2Client struct {
	describeInstancesOutput *ec2.DescribeInstancesOutput
	runInstancesOutput     *ec2.Reservation
	err                    error
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

	_, err = db.Exec(`
		CREATE TABLE launch_requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			instance_id TEXT,
			desired_state TEXT,
			current_state TEXT
		)
	`)
	if err != nil {
		t.Fatalf("Error creating test launch_requests table: %s", err)
	}

	return db, dbPath
}

func TestGetInstanceState(t *testing.T) {
	testInstanceManager := &instanceManager{
		db: nil,
		ec2Client: &mockEC2Client{
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
		},
	}

	instanceInfo, err := testInstanceManager.getInstanceState("i-1234567890abcdef0")
	if err != nil {
		t.Fatalf("Error getting instance state: %v", err)
	}
	want := &InstanceInfo{
		InstanceType: "t3.micro",
		AMI:         "ami-1234567890abcdef0",
		State:       "running",
	}

	if !reflect.DeepEqual(instanceInfo, want) {
		t.Errorf("got %v, want %v", instanceInfo, want)
	}
}

func TestLaunchInstance(t *testing.T) {
	testInstanceManager := &instanceManager{
		db: nil,
		ec2Client: &mockEC2Client{
			runInstancesOutput: &ec2.Reservation{
				Instances: []*ec2.Instance{{
					InstanceId: aws.String("i-1234567890abcdef0"),
				}},
			},
		},
	}

	instanceID, err := testInstanceManager.launchInstance("t3.micro", "ami-1234567890abcdef0")
	if err != nil {
		t.Fatalf("Error launching instance: %v", err)
	}

	want := "i-1234567890abcdef0"
	if instanceID != want {
		t.Errorf("got %q, want %q", instanceID, want)
	}
}

func TestUpdateCurrentState(t *testing.T) {
	db, dbPath := setupTestDB(t)
	defer db.Close()
	defer os.Remove(dbPath)

	testInstanceManager := &instanceManager{
		db: db,
		ec2Client: &mockEC2Client{
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
		},
	}

	_, err := db.Exec(
		`INSERT INTO launch_requests (id, instance_id, desired_state, current_state) VALUES (?, ?, ?, ?)`,
		"1", "i-1234567890abcdef0", sampleDesiredState, nil,
	)
	if err != nil {
		t.Fatalf("Error inserting test launch_requests row: %s", err)
	}

	testInstanceManager.updateCurrentState()

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
	db, dbPath := setupTestDB(t)
	defer db.Close()
	defer os.Remove(dbPath)

	testInstanceManager := &instanceManager{
		db: db,
		ec2Client: &mockEC2Client{
			runInstancesOutput: &ec2.Reservation{
				Instances: []*ec2.Instance{{
					InstanceId: aws.String("i-1234567890abcdef1"),
				}},
			},
		},
	}

	_, err := db.Exec(
		`INSERT INTO launch_requests (id, instance_id, desired_state, current_state) VALUES (?, ?, ?, ?)`,
		"1", nil, sampleDesiredState, nil,
	)
	if err != nil {
		t.Fatalf("Error inserting test launch_requests row: %s", err)
	}

	processLaunchRequests(testInstanceManager)

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

func TestHandleLaunchRequest(t *testing.T) {
	db, dbPath := setupTestDB(t)
	defer db.Close()
	defer os.Remove(dbPath)

	ec2Client := &mockEC2Client{
		runInstancesOutput: &ec2.Reservation{
			Instances: []*ec2.Instance{{
				InstanceId: aws.String("i-1234567890abcdef1"),
			}},
		},
	}

	handleLaunchRequest(db, nil, &http.Request{
		Body: io.NopCloser(strings.NewReader(sampleDesiredState)),
	}, ec2Client)

	// wait for the goroutine to finish
	time.Sleep(5 * time.Second)

	// check how many rows are in the launch_requests table
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM launch_requests`).Scan(&count)
	if err != nil {
		t.Fatalf("Error querying launch_requests table: %s", err)
	}

	want_count := 1
	if count != want_count {
		t.Errorf("got %d, want %d", count, want_count)
	}

	// check that instance_id is set on that row
	var instanceID string
	err = db.QueryRow(`SELECT instance_id FROM launch_requests WHERE id = ?`, "1").Scan(&instanceID)
	if err != nil {
		t.Fatalf("Error querying instance_id column: %s", err)
	}
	want_instanceID := "i-1234567890abcdef1"
	if instanceID != want_instanceID {
		t.Errorf("got %q, want %q", instanceID, want_instanceID)
	}
}
