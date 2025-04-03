package reefd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ray-project/rayci/reefd/reefapi"
)

type session struct {
	user   string
	token  string
	expire time.Time
}

type authGate struct {
	rand io.Reader

	nowFunc  func() time.Time
	userKeys map[string]string

	mu       sync.Mutex
	sessions map[string]*session
}

func newAuthGate(userKeys map[string]string) *authGate {
	return &authGate{
		rand:     rand.Reader,
		nowFunc:  time.Now,
		sessions: make(map[string]*session),
		userKeys: userKeys,
	}
}

const sessionTokenPrefix = "ses_"

func (g *authGate) newSessionToken(req *reefapi.TokenRequest) (string, error) {
	rand := make([]byte, 32)
	if _, err := io.ReadFull(g.rand, rand); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	// Encode the token in base64 to make it URL safe.
	token := sessionTokenPrefix + base64.RawURLEncoding.EncodeToString(rand)
	now := g.nowFunc()
	const ttl = 10 * time.Hour

	g.mu.Lock()
	defer g.mu.Unlock()

	g.sessions[token] = &session{
		user:   req.User,
		token:  token,
		expire: now.Add(ttl),
	}

	return token, nil
}

func (g *authGate) apiLogin(_ context.Context, req *reefapi.LoginRequest) (
	*reefapi.LoginResponse, error,
) {
	if req.User == "" {
		return nil, fmt.Errorf("user is empty")
	}

	keyBytes, ok := g.userKeys[req.User]
	if !ok {
		return nil, fmt.Errorf("user %q not found", req.User)
	}

	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keyBytes))
	if err != nil {
		return nil, fmt.Errorf(
			"parse public key of user %q: %w", req.User, err,
		)
	}

	if req.SigningMethod != "ssh-ed25519" {
		return nil, fmt.Errorf("unsupported signing method %q", req.SigningMethod)
	}

	sig := new(reefapi.SSHSignature)
	if err := json.Unmarshal(req.Signature, sig); err != nil {
		return nil, fmt.Errorf("unmarshal signature: %w", err)
	}

	sshSig := &ssh.Signature{
		Format: sig.Format,
		Blob:   sig.Blob,
		Rest:   sig.Rest,
	}

	if err := pubKey.Verify(req.TokenRequest, sshSig); err != nil {
		return nil, fmt.Errorf("verify signature: %w", err)
	}

	tokenReq := new(reefapi.TokenRequest)
	if err := json.Unmarshal(req.TokenRequest, tokenReq); err != nil {
		return nil, fmt.Errorf("unmarshal token request: %w", err)
	}

	if tokenReq.User != req.User {
		return nil, fmt.Errorf(
			"user mismatch: %q != %q",
			tokenReq.User, req.User,
		)
	}

	sessionToken, err := g.newSessionToken(tokenReq)
	if err != nil {
		return nil, fmt.Errorf("new session token: %w", err)
	}

	resp := &reefapi.LoginResponse{SessionToken: sessionToken}
	return resp, nil
}

func (g *authGate) check(sessionToken string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// TODO(aslonnie): this is going to be a performance bottleneck.
	ses, ok := g.sessions[sessionToken]
	if !ok {
		return "", fmt.Errorf("session %q not found", sessionToken)
	}

	if g.nowFunc().After(ses.expire) {
		delete(g.sessions, sessionToken)
		return "", fmt.Errorf("session %q expired", sessionToken)
	}

	return ses.user, nil
}

func (g *authGate) apiLogout(_ context.Context, req *reefapi.LogoutRequest) (
	*reefapi.LogoutResponse, error,
) {
	if req.SessionToken == "" {
		return nil, fmt.Errorf("session token is empty")
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.sessions, req.SessionToken)

	return &reefapi.LogoutResponse{}, nil
}
