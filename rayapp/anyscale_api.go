package rayapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// AnyscaleAPI handles HTTP client calls to the Anyscale API host.
type AnyscaleAPI struct {
	host   string
	token  string
	client *http.Client
}

// NewAnyscaleAPI creates an AnyscaleAPI with the given host and token.
func NewAnyscaleAPI(host, token string) (*AnyscaleAPI, error) {
	if host == "" {
		return nil, errors.New("host is empty")
	}
	if token == "" {
		return nil, errors.New("token is empty")
	}
	return &AnyscaleAPI{
		host:   host,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// DeleteWorkspaceByID deletes a workspace by its ID using the Anyscale REST API.
func (a *AnyscaleAPI) DeleteWorkspaceByID(workspaceID string) error {
	if workspaceID == "" {
		return errors.New("workspace ID is empty")
	}
	reqURL, err := url.JoinPath(
		a.host, "/api/v2/experimental_workspaces", url.PathEscape(workspaceID),
	)
	if err != nil {
		return fmt.Errorf("failed to construct workspace URL: %w", err)
	}

	req, err := http.NewRequest(http.MethodDelete, reqURL, nil)
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

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		return fmt.Errorf(
			"delete workspace failed with status %d: %s",
			resp.StatusCode,
			string(body),
		)
	}

	fmt.Printf("delete workspace %s succeeded\n", workspaceID)
	return nil
}
