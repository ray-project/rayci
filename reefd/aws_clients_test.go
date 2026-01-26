package reefd

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
)

func TestNewAWSClientsFromConfig(t *testing.T) {
	cfg := &aws.Config{
		Region: "us-west-2",
	}

	clients := newAWSClientsFromConfig(cfg)
	if clients == nil {
		t.Fatal("newAWSClientsFromConfig() returned nil")
	}
	if clients.ec2 == nil {
		t.Error("ec2 client factory is nil")
	}

	// Verify the factory produces a client (doesn't panic)
	ec2Client := clients.ec2()
	if ec2Client == nil {
		t.Error("ec2() returned nil")
	}
}
