package reefclient

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/ray-project/rayci/reefd/reefapi"
	"golang.org/x/crypto/ssh"
)

type client struct {
	caller *JSONCaller
}

func newClient(server string) (*client, error) {
	caller, err := NewJSONCaller(server)
	if err != nil {
		return nil, fmt.Errorf("new caller: %w", err)
	}
	return &client{caller: caller}, nil
}

func (c *client) callLogin(ctx context.Context, req *reefapi.LoginRequest) (
	*reefapi.LoginResponse, error,
) {
	resp := &reefapi.LoginResponse{}
	if err := JSONCall(ctx, c.caller, "api/v1/login", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *client) callLogout(ctx context.Context, req *reefapi.LogoutRequest) (
	*reefapi.LogoutResponse, error,
) {
	resp := &reefapi.LogoutResponse{}
	if err := JSONCall(ctx, c.caller, "api/v1/logout", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *client) login(ctx context.Context, user string) (string, error) {
	tokenReq := &reefapi.TokenRequest{User: user}

	tokenReqBytes, err := json.Marshal(tokenReq)
	if err != nil {
		return "", fmt.Errorf("encode token request: %w", err)
	}
	privateKeyFile := os.ExpandEnv("$HOME/.ssh/id_ed25519")
	privateKeyBytes, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return "", fmt.Errorf("read private key: %w", err)
	}

	priKey, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("parse private key: %w", err)
	}

	sshSig, err := priKey.Sign(rand.Reader, tokenReqBytes)
	if err != nil {
		return "", fmt.Errorf("sign token request: %w", err)
	}
	sigBytes, err := json.Marshal(&reefapi.SSHSignature{
		Format: sshSig.Format,
		Blob:   sshSig.Blob,
		Rest:   sshSig.Rest,
	})
	if err != nil {
		return "", fmt.Errorf("encode signature: %w", err)
	}

	resp, err := c.callLogin(ctx, &reefapi.LoginRequest{
		User:          user,
		TokenRequest:  tokenReqBytes,
		SigningMethod: "ssh-ed25519",
		Signature:     sigBytes,
	})
	if err != nil {
		return "", fmt.Errorf("login: %w", err)
	}

	return resp.SessionToken, nil
}

func (c *client) logout(ctx context.Context, sessionToken string) error {
	logoutReq := &reefapi.LogoutRequest{
		SessionToken: sessionToken,
	}
	if _, err := c.callLogout(ctx, logoutReq); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// Main is the main function that runs the client.
func Main(ctx context.Context) error {
	const server = "http://localhost:8000"

	client, err := newClient(server)
	if err != nil {
		return fmt.Errorf("new client: %w", err)
	}

	const user = "aslonnie"

	tok, err := client.login(ctx, user)
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	log.Printf("session token: %q", tok)

	if err := client.logout(ctx, tok); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	log.Println("successfully logout")

	return nil
}
