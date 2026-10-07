package rayapp

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetDefaultCloud(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{
			defaultCloud: &fakeCloud{
				Name: "my-default-cloud", ID: "cld_abc123",
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"cloud", "get-default"}, nil, nil,
			)
			return fake.run(args)
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
			checkArgs(t, args,
				[]string{"cloud", "get-default"}, nil, nil,
			)
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
			checkArgs(t, args,
				[]string{"cloud", "get-default"}, nil, nil,
			)
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
			checkArgs(t, args,
				[]string{"cloud", "get-default"}, nil, nil,
			)
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

func TestGetCloud(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{
			clouds: []*fakeCloud{
				{Name: "other-cloud", ID: "cld_other"},
				{Name: "k8s-cloud", ID: "cld_k8s"},
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"cloud", "get"}, nil,
				[][2]string{{"--name", "k8s-cloud"}},
			)
			return fake.run(args)
		})

		cloudInfo, err := cli.GetCloud("k8s-cloud")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cloudInfo.Name != "k8s-cloud" || cloudInfo.ID != "cld_k8s" {
			t.Errorf("CloudInfo = %+v, want {k8s-cloud cld_k8s}", *cloudInfo)
		}
	})

	t.Run("not found", func(t *testing.T) {
		cli := newTestCLI(&fakeAnyscale{})

		_, err := cli.GetCloud("missing-cloud")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `cloud "missing-cloud" not found`) {
			t.Errorf("error %q should say the cloud was not found", err.Error())
		}
	})

	t.Run("CLI failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.GetCloud("k8s-cloud")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), `get cloud "k8s-cloud" failed`) {
			t.Errorf("error %q should contain 'get cloud \"k8s-cloud\" failed'", err.Error())
		}
	})
}
