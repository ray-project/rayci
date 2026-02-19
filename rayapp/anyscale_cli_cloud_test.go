package rayapp

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetDefaultCloud(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "name: my-default-cloud\nid: cld_abc123\n", nil
		})

		cloudInfo, err := cli.GetDefaultCloud()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cloudInfo == nil {
			t.Fatal("expected CloudInfo, got nil")
		}
		if cloudInfo.Name != "my-default-cloud" {
			t.Errorf("CloudInfo.Name = %q, want %q", cloudInfo.Name, "my-default-cloud")
		}
		if cloudInfo.ID != "cld_abc123" {
			t.Errorf("CloudInfo.ID = %q, want %q", cloudInfo.ID, "cld_abc123")
		}
	})

	t.Run("CLI failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.GetDefaultCloud()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "get default cloud failed") {
			t.Errorf("error %q should contain 'get default cloud failed'", err.Error())
		}
	})

	t.Run("invalid YAML output", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "invalid: yaml: output: [", nil
		})

		_, err := cli.GetDefaultCloud()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "failed to parse cloud info") {
			t.Errorf("error %q should contain 'failed to parse cloud info'", err.Error())
		}
	})

	t.Run("empty output", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "", nil
		})

		cloudInfo, err := cli.GetDefaultCloud()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cloudInfo.Name != "" {
			t.Errorf("CloudInfo.Name = %q, want empty string", cloudInfo.Name)
		}
		if cloudInfo.ID != "" {
			t.Errorf("CloudInfo.ID = %q, want empty string", cloudInfo.ID)
		}
	})
}
