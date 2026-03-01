package rayapp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
)

// apiError represents a non-2xx HTTP response from the Anyscale API.
type apiError struct {
	StatusCode int
	Body       string
}

func (e *apiError) Error() string {
	return fmt.Sprintf(
		"request failed with status %d: %s",
		e.StatusCode, e.Body,
	)
}

var validWorkspaceID = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// anyscaleAPI handles HTTP client calls to the Anyscale API host.
type anyscaleAPI struct {
	host   string
	token  string
	client *http.Client
}

func newAnyscaleAPI(host, token string) (*anyscaleAPI, error) {
	if host == "" {
		return nil, errors.New("host is empty")
	}
	if token == "" {
		return nil, errors.New("token is empty")
	}
	return &anyscaleAPI{host: host, token: token, client: &http.Client{}}, nil
}

func (a *anyscaleAPI) deleteWorkspaceByID(workspaceID string) error {
	if !validWorkspaceID.MatchString(workspaceID) {
		return fmt.Errorf("invalid workspace ID: %q", workspaceID)
	}
	reqURL, err := url.JoinPath(
		a.host, "api/v2/experimental_workspaces", workspaceID,
	)
	if err != nil {
		return fmt.Errorf("construct workspace URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(
			io.LimitReader(resp.Body, 1024),
		)
		if err != nil {
			return fmt.Errorf(
				"read response body: %w", err,
			)
		}
		return &apiError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}
	io.Copy(io.Discard, resp.Body)

	return nil
}

func (a *anyscaleAPI) launchTemplateInWorkspace(
	cloudID, projectID, templateName string, workspaceName string,
) (map[string]any, error) {
	if cloudID == "" {
		return nil, errors.New("cloudID is empty")
	}
	if projectID == "" {
		return nil, errors.New("projectID is empty")
	}
	if templateName == "" {
		return nil, errors.New("templateName is empty")
	}
	reqURL, err := url.JoinPath(a.host, "api/v2/experimental_workspaces/from_template")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	payload := map[string]any{
		"template_id": templateName,
		"name":        workspaceName,
		"cloud_id":    cloudID,
		"project_id":  projectID,
	}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewBuffer(jsonPayload))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(
		io.LimitReader(resp.Body, 1024*1024),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read response body: %w", err,
		)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response body: %w", err)
	}

	result, ok := response["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected response format: missing 'result' key")
	}

	return result, nil
}
