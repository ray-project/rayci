package rayapp

import (
	"encoding/json"
	"fmt"
	"strings"
)

type fakeCloud struct {
	Name string
	ID   string
}

type fakeProject struct {
	Name string
	ID   string
}

type fakeComputeConfig struct {
	ID             string
	Name           string
	CloudID        string
	Version        int
	CreatedAt      string
	LastModifiedAt string
	URL            string
}

type fakeWorkspace struct {
	ID    string
	Name  string
	State string
}

// fakeAnyscale simulates the Anyscale CLI for tests. It dispatches
// subcommands based on its fake data fields.
type fakeAnyscale struct {
	defaultCloud   *fakeCloud
	defaultProject *fakeProject
	computeConfigs []*fakeComputeConfig
	workspaces     []*fakeWorkspace

	// commandErrors, if set, maps "group subcommand" keys (e.g.
	// "workspace_v2 create") to errors returned before normal dispatch.
	commandErrors map[string]error

	// onCreateComputeConfig, if set, is called for "compute-config create"
	// with the full args slice. If nil, create succeeds with a generic message.
	onCreateComputeConfig func(args []string) (string, error)

	// onCreateWorkspace, if set, is called for "workspace_v2 create"
	// with the full args slice. If nil, create succeeds with a generic message.
	onCreateWorkspace func(args []string) (string, error)
}

func (f *fakeAnyscale) run(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("fake: insufficient args: %v", args)
	}
	cmd := fmt.Sprintf("%s %s", args[0], args[1])
	if f.commandErrors != nil {
		if err, ok := f.commandErrors[cmd]; ok {
			return "", err
		}
	}
	switch cmd {
	case "cloud get-default":
		return f.cloudGetDefault()
	case "project get-default":
		return f.projectGetDefault()
	case "compute-config list":
		return f.computeConfigList(args[2:])
	case "compute-config create":
		if f.onCreateComputeConfig != nil {
			return f.onCreateComputeConfig(args)
		}
		return "created compute config", nil
	case "workspace_v2 create":
		if f.onCreateWorkspace != nil {
			return f.onCreateWorkspace(args)
		}
		return f.workspaceCreate(args[2:])
	case "workspace_v2 get":
		return f.workspaceGet(args[2:])
	case "workspace_v2 terminate":
		return f.workspaceTerminate(args[2:])
	case "workspace_v2 push":
		return f.workspacePush(args[2:])
	case "workspace_v2 run_command":
		return f.workspaceRunCommand(args[2:])
	case "workspace_v2 start":
		return f.workspaceStart(args[2:])
	case "workspace_v2 status":
		return f.workspaceStatus(args[2:])
	case "workspace_v2 wait":
		return f.workspaceWait(args[2:])
	default:
		return "", fmt.Errorf("fake: unknown command: %v", args)
	}
}

func (f *fakeAnyscale) cloudGetDefault() (string, error) {
	if f.defaultCloud == nil {
		return "", fmt.Errorf("no default cloud configured")
	}
	return strings.Join([]string{
		fmt.Sprintf("name: %s", f.defaultCloud.Name),
		fmt.Sprintf("id: %s", f.defaultCloud.ID),
		"",
	}, "\n"), nil
}

func (f *fakeAnyscale) projectGetDefault() (string, error) {
	if f.defaultProject == nil {
		return "", fmt.Errorf("no default project configured")
	}
	return strings.Join([]string{
		fmt.Sprintf("name: %s", f.defaultProject.Name),
		fmt.Sprintf("id: %s", f.defaultProject.ID),
		"",
	}, "\n"), nil
}

func parseFlag(opts []string, flags ...string) string {
	flagSet := make(map[string]struct{}, len(flags))
	for _, f := range flags {
		flagSet[f] = struct{}{}
	}
	for i := 0; i < len(opts)-1; i++ {
		if _, ok := flagSet[opts[i]]; ok {
			return opts[i+1]
		}
	}
	return ""
}

func parseName(opts []string) string {
	return parseFlag(opts, "--name", "-n")
}

func (f *fakeAnyscale) workspaceCreate(opts []string) (string, error) {
	name := parseName(opts)
	id := fmt.Sprintf("expwrk_%s", name)
	f.workspaces = append(f.workspaces, &fakeWorkspace{
		ID: id, Name: name, State: "TERMINATED",
	})
	return fmt.Sprintf("Workspace created successfully id: %s", id), nil
}

func (f *fakeAnyscale) workspaceGet(opts []string) (string, error) {
	name := parseName(opts)
	for _, ws := range f.workspaces {
		if ws.Name == name {
			m := map[string]any{
				"id":    ws.ID,
				"name":  ws.Name,
				"state": ws.State,
			}
			bs, err := json.Marshal(m)
			if err != nil {
				return "", fmt.Errorf("fake: marshal: %w", err)
			}
			return string(bs), nil
		}
	}
	return "", fmt.Errorf("fake: workspace not found: %s", name)
}

func (f *fakeAnyscale) workspaceTerminate(opts []string) (string, error) {
	name := parseName(opts)
	return fmt.Sprintf("Terminating workspace '%s'", name), nil
}

func (f *fakeAnyscale) workspacePush(opts []string) (string, error) {
	name := parseName(opts)
	localDir := parseFlag(opts, "--local-dir")
	return fmt.Sprintf("Sending %s to workspace '%s'", localDir, name), nil
}

func (f *fakeAnyscale) workspaceRunCommand(opts []string) (string, error) {
	name := parseName(opts)
	return fmt.Sprintf("Running command in workspace '%s'", name), nil
}

func (f *fakeAnyscale) workspaceStart(opts []string) (string, error) {
	name := parseName(opts)
	return fmt.Sprintf("Starting workspace '%s'", name), nil
}

func (f *fakeAnyscale) workspaceStatus(opts []string) (string, error) {
	name := parseName(opts)
	for _, ws := range f.workspaces {
		if ws.Name == name {
			return ws.State, nil
		}
	}
	return "", fmt.Errorf("fake: workspace not found: %s", name)
}

func (f *fakeAnyscale) workspaceWait(opts []string) (string, error) {
	name := parseName(opts)
	state := parseFlag(opts, "--state")
	return strings.Join([]string{
		fmt.Sprintf(
			"Waiting for workspace '%s' to reach target state %s, currently in state: %s",
			name, state, state,
		),
		fmt.Sprintf("Workspace '%s' reached target state, exiting", name),
	}, "\n"), nil
}

func (f *fakeAnyscale) computeConfigList(opts []string) (string, error) {
	nameFilter := parseName(opts)

	var results []map[string]any
	for _, cc := range f.computeConfigs {
		if nameFilter != "" && cc.Name != nameFilter {
			continue
		}
		results = append(results, map[string]any{
			"id":               cc.ID,
			"name":             cc.Name,
			"cloud_id":         cc.CloudID,
			"version":          cc.Version,
			"created_at":       cc.CreatedAt,
			"last_modified_at": cc.LastModifiedAt,
			"url":              cc.URL,
		})
	}
	if results == nil {
		results = []map[string]any{}
	}

	resp := map[string]any{
		"results": results,
		"metadata": map[string]any{
			"count":      len(results),
			"next_token": nil,
		},
	}
	bs, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("fake: marshal: %w", err)
	}
	return string(bs), nil
}
