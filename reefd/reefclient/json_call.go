package reefclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

// JSONCaller is a simple HTTP client that sends JSON requests and
// receives JSON responses. It is used to communicate with the REEf
// server.
type JSONCaller struct {
	client    *http.Client
	server    *url.URL
	agentName string
}

// DefaultAgentName is the default agent name used in the User-Agent.
const DefaultAgentName = "reefy"

// JSONContentType is the content type for JSON requests and
// responses.
const JSONContentType = "application/json"

// NewJSONCaller creates a new JSONCaller with the given server URL.
func NewJSONCaller(server string) (*JSONCaller, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if u.Path == "" {
		u.Path = "/"
	}

	return &JSONCaller{
		client:    &http.Client{},
		server:    u,
		agentName: DefaultAgentName,
	}, nil
}

// call makes a JSON call to the server with the given path and
// request body. It returns the response body or an error.
func (c *JSONCaller) call(ctx context.Context, p string, req []byte) (
	[]byte, error,
) {
	u := *c.server
	u.Path = path.Join(u.Path, p)

	h := http.Header{}
	const contentType = JSONContentType
	h.Set("Accept", contentType)
	h.Set("Content-Type", contentType)
	if c.agentName != "" {
		h.Set("User-Agent", c.agentName)
	}

	httpReq := &http.Request{
		Method: http.MethodPost,
		URL:    &u,
		Header: h,
		Body:   io.NopCloser(bytes.NewReader(req)),
	}
	httpReq = httpReq.WithContext(ctx)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				return
			}
		}()

		buf := new(bytes.Buffer)
		const maxErrBodySize = 1024
		_, err := io.CopyN(buf, resp.Body, maxErrBodySize)
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf(
				"http error %s, and while read body: %w",
				resp.Status, err,
			)
		}

		if buf.Len() > 0 {
			return nil, fmt.Errorf(
				"http error %s: %s", resp.Status, buf.String(),
			)
		}
		return nil, fmt.Errorf("http error %s", resp.Status)
	}

	if t := resp.Header.Get("Content-Type"); t != contentType {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				return
			}
		}()
		return nil, fmt.Errorf("got content type %q, want %q", t, contentType)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	return body, nil
}

// JSONCall makes a JSON call to the server with the given path,
// request, and response. It encodes the request into JSON and decodes
// the response from JSON. It returns an error if any step fails.
func JSONCall[R, S any](
	ctx context.Context, c *JSONCaller, p string, req *R, resp *S,
) error {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	respBytes, err := c.call(ctx, p, reqBytes)
	if err != nil {
		return fmt.Errorf("call: %w", err)
	}

	dec := json.NewDecoder(bytes.NewReader(respBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(resp); err != nil {
		return fmt.Errorf(
			"decode response: %w - response %q",
			err, respBytes,
		)
	}
	return nil
}
