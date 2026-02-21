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

type fakeComputeConfig struct {
	ID             string
	Name           string
	CloudID        string
	Version        int
	CreatedAt      string
	LastModifiedAt string
	URL            string
}

// fakeAnyscale simulates the Anyscale CLI for tests. It dispatches
// "cloud get-default", "compute-config list", and "compute-config create"
// based on its fake data fields.
type fakeAnyscale struct {
	defaultCloud   *fakeCloud
	computeConfigs []*fakeComputeConfig

	// onCreateComputeConfig, if set, is called for "compute-config create"
	// with the full args slice. If nil, create succeeds with a generic message.
	onCreateComputeConfig func(args []string) (string, error)
}

func (f *fakeAnyscale) run(args []string) (string, error) {
	if len(args) < 2 {
		return "", fmt.Errorf("fake: insufficient args: %v", args)
	}
	switch args[0] + " " + args[1] {
	case "cloud get-default":
		return f.cloudGetDefault()
	case "compute-config list":
		return f.computeConfigList(args[2:])
	case "compute-config create":
		if f.onCreateComputeConfig != nil {
			return f.onCreateComputeConfig(args)
		}
		return "created compute config", nil
	default:
		return "", fmt.Errorf("fake: unknown command: %v", args)
	}
}

func (f *fakeAnyscale) cloudGetDefault() (string, error) {
	if f.defaultCloud == nil {
		return "", fmt.Errorf("no default cloud configured")
	}
	return strings.Join([]string{
		"name: " + f.defaultCloud.Name,
		"id: " + f.defaultCloud.ID,
		"",
	}, "\n"), nil
}

func (f *fakeAnyscale) computeConfigList(opts []string) (string, error) {
	var nameFilter string
	for i := 0; i < len(opts)-1; i++ {
		if opts[i] == "--name" || opts[i] == "-n" {
			nameFilter = opts[i+1]
			break
		}
	}

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
