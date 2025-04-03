package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/ray-project/rayci/reefapi"
	"golang.org/x/crypto/ssh"
)

type jsonCaller struct {
	client    *http.Client
	server    *url.URL
	agentName string
}

func newJSONCaller(server string) (*jsonCaller, error) {
	u, err := url.Parse(server)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if u.Path == "" {
		u.Path = "/"
	}

	return &jsonCaller{
		client:    &http.Client{},
		server:    u,
		agentName: "reefy",
	}, nil
}

func (c *jsonCaller) call(ctx context.Context, p string, req []byte) (*http.Response, error) {
	u := *c.server
	u.Path = path.Join(u.Path, p)

	h := http.Header{}
	const jsonContentType = "application/json"
	const agentName = "reefy"
	h.Set("Accept", jsonContentType)
	h.Set("Content-Type", jsonContentType)
	h.Set("User-Agent", c.agentName)

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
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("read response body: %w", err)
		}
		return nil, fmt.Errorf("http error %s: %s", resp.Status, body)
	}

	if t := resp.Header.Get("Content-Type"); t != jsonContentType {
		defer resp.Body.Close()
		return nil, fmt.Errorf("got content type %q, want %q", t, jsonContentType)
	}

	return resp, nil
}

func jsonCall[R, S any](ctx context.Context, caller *jsonCaller, p string, req *R, resp *S) error {
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	httpResp, err := caller.call(ctx, p, reqBytes)
	if err != nil {
		return fmt.Errorf("call: %w", err)
	}
	defer httpResp.Body.Close()

	respBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
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

func run(ctx context.Context) error {
	caller, err := newJSONCaller("http://localhost:8000")
	if err != nil {
		return fmt.Errorf("new JSON caller: %w", err)
	}

	const user = "aslonnie"
	tokenReq := &reefapi.TokenRequest{User: user}

	tokenReqBytes, err := json.Marshal(tokenReq)
	if err != nil {
		return fmt.Errorf("encode token request: %w", err)
	}
	privateKeyFile := os.ExpandEnv("$HOME/.ssh/id_ed25519")
	privateKeyBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return fmt.Errorf("read private key: %w", err)
	}

	priKey, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return fmt.Errorf("parse private key: %w", err)
	}

	sshSig, err := priKey.Sign(rand.Reader, tokenReqBytes)
	if err != nil {
		return fmt.Errorf("sign token request: %w", err)
	}
	sigBytes, err := json.Marshal(&reefapi.SSHSignature{
		Format: sshSig.Format,
		Blob:   sshSig.Blob,
		Rest:   sshSig.Rest,
	})
	if err != nil {
		return fmt.Errorf("encode signature: %w", err)
	}

	resp := &reefapi.LoginResponse{}
	if err := jsonCall(ctx, caller, "api/v1/login", &reefapi.LoginRequest{
		User:          user,
		TokenRequest:  tokenReqBytes,
		SigningMethod: "ssh-ed25519",
		Signature:     sigBytes,
	}, resp); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	log.Printf("session token: %q", resp.SessionToken)

	logoutReq := &reefapi.LogoutRequest{
		SessionToken: resp.SessionToken,
	}
	logoutResp := &reefapi.LogoutResponse{}
	if err := jsonCall(ctx, caller, "api/v1/logout", logoutReq, logoutResp); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	log.Println("logout successful")

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}
