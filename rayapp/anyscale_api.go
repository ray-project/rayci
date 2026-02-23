package rayapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// AnyscaleAPI handles HTTP client calls to the Anyscale API host.
type AnyscaleAPI struct {
	host   string
	token  string
	client *http.Client
}

func newAnyscaleAPI() (*AnyscaleAPI, error) {
	host := os.Getenv("ANYSCALE_HOST")
	if host == "" {
		return nil, errors.New("ANYSCALE_HOST environment variable is not set")
	}
	token := os.Getenv("ANYSCALE_CLI_TOKEN")
	if token == "" {
		return nil, errors.New("ANYSCALE_CLI_TOKEN environment variable is not set")
	}
	return &AnyscaleAPI{host: host, token: token, client: &http.Client{}}, nil
}

// DeleteWorkspaceByID deletes a workspace by its ID using the Anyscale REST API.
func (a *AnyscaleAPI) DeleteWorkspaceByID(workspaceID string) error {
	reqURL, err := url.JoinPath(a.host, "api/v2/experimental_workspaces", workspaceID)
	if err != nil {
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf(
			"delete workspace failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	fmt.Printf("delete workspace %s succeeded: %s\n", workspaceID, string(body))
	return nil
}

func (a *AnyscaleAPI) LaunchTemplateInWorkspace(cloudID string, projectID string, templateName string) (map[string]any, error) {
	reqURL, err := url.JoinPath(a.host, "api/v2/experimental_workspaces/from_template")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	payload := map[string]any{
		"template_id": templateName,
		"name":        slugify(templateName) + "-" + time.Now().Format("20060102150405"),
		"cloud_id":    cloudID,
		"project_id":  projectID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf(
			"launch template in workspace failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	result, ok := response["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' key")
	}

	return result, nil
}