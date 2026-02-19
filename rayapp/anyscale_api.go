package rayapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// AnyscaleAPI handles HTTP client calls to the Anyscale API host.
type AnyscaleAPI struct {
	host   string
	token  string
	client *http.Client
}

// NewAnyscaleAPI creates an AnyscaleAPI from environment variables.
func NewAnyscaleAPI() (*AnyscaleAPI, error) {
	host := os.Getenv("ANYSCALE_HOST")
	if host == "" {
		return nil, errors.New("ANYSCALE_HOST environment variable is not set")
	}
	token := os.Getenv("ANYSCALE_CLI_TOKEN")
	if token == "" {
		return nil, errors.New("ANYSCALE_CLI_TOKEN environment variable is not set")
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
	reqURL := fmt.Sprintf(
		"%s/api/v2/experimental_workspaces/%s",
		a.host, url.PathEscape(workspaceID),
	)

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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
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
