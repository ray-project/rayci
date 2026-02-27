package rayapp

import (
	"fmt"
	"strings"
	"testing"
)

func TestGetDefaultProject(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		fake := &fakeAnyscale{
			defaultProject: &fakeProject{
				Name: "my-default-project",
				ID:   "prj_abc123",
			},
		}
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"project", "get-default"},
				nil,
				[][2]string{{"--cloud", "cld_abc123"}},
			)
			return fake.run(args)
		})

		projectInfo, err := cli.GetDefaultProject("cld_abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectInfo == nil {
			t.Fatal("expected ProjectInfo, got nil")
		}
		if projectInfo.Name != "my-default-project" {
			t.Errorf(
				"ProjectInfo.Name = %q, want %q",
				projectInfo.Name, "my-default-project",
			)
		}
		if projectInfo.ID != "prj_abc123" {
			t.Errorf(
				"ProjectInfo.ID = %q, want %q",
				projectInfo.ID, "prj_abc123",
			)
		}
	})

	t.Run("CLI failure", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"project", "get-default"},
				nil,
				[][2]string{{"--cloud", "cld_abc123"}},
			)
			return "", fmt.Errorf("exit status 1")
		})

		_, err := cli.GetDefaultProject("cld_abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(), "get default Project failed",
		) {
			t.Errorf(
				"error %q should contain "+
					"'get default Project failed'",
				err.Error(),
			)
		}
	})

	t.Run("invalid YAML output", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"project", "get-default"},
				nil,
				[][2]string{{"--cloud", "cld_abc123"}},
			)
			return "invalid: yaml: output: [", nil
		})

		_, err := cli.GetDefaultProject("cld_abc123")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(
			err.Error(), "failed to parse project info",
		) {
			t.Errorf(
				"error %q should contain "+
					"'failed to parse project info'",
				err.Error(),
			)
		}
	})

	t.Run("empty output", func(t *testing.T) {
		cli := NewAnyscaleCLI()
		cli.setRunFunc(func(args []string) (string, error) {
			checkArgs(t, args,
				[]string{"project", "get-default"},
				nil,
				[][2]string{{"--cloud", "cld_abc123"}},
			)
			return "", nil
		})

		projectInfo, err := cli.GetDefaultProject("cld_abc123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if projectInfo.Name != "" {
			t.Errorf(
				"ProjectInfo.Name = %q, want empty string",
				projectInfo.Name,
			)
		}
		if projectInfo.ID != "" {
			t.Errorf(
				"ProjectInfo.ID = %q, want empty string",
				projectInfo.ID,
			)
		}
	})
}
