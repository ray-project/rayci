package rayapp

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"time"
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
	return &anyscaleAPI{
		host:   host,
		token:  token,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (a *anyscaleAPI) deleteWorkspaceByID(workspaceID string) error {
	if !validWorkspaceID.MatchString(workspaceID) {
		return fmt.Errorf("invalid workspace ID: %q", workspaceID)
	}
	reqURL, err := url.JoinPath(
		a.host, "/api/v2/experimental_workspaces", workspaceID,
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
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
		if err != nil {
			return fmt.Errorf("read response body: %w", err)
		}
		return &apiError{
			StatusCode: resp.StatusCode,
			Body:       string(body),
		}
	}

	io.Copy(io.Discard, resp.Body)
	return nil
}
