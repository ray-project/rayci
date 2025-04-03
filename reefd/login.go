package reefd

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	unauth map[string]struct{}
}

func newAuthGate(userKeys map[string]string, unauth []string) *authGate {
	unauthMap := make(map[string]struct{}, len(unauth))
	for _, u := range unauth {
		unauthMap[u] = struct{}{}
	}

	return &authGate{
		rand:     rand.Reader,
		nowFunc:  time.Now,
		sessions: make(map[string]*session),
		userKeys: userKeys,
		unauth:   unauthMap,
	}
}

const sessionTokenPrefix = "ses_"

func (g *authGate) newSessionToken(req *reefapi.TokenRequest) (string, error) {
	const tokenSize = 24
	rand := make([]byte, tokenSize)
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
		return nil, fmt.Errorf(
			"unsupported signing method %q",
			req.SigningMethod,
		)
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

func (g *authGate) gate(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := g.unauth[r.URL.Path]; ok {
			// unauth endpoints are not protected.
			h.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "authorization header is empty", http.StatusUnauthorized)
			return
		}

		const bearerPrefix = "Bearer "

		if !strings.HasPrefix(authHeader, bearerPrefix) {
			http.Error(w, "authorization header must be Bearer", http.StatusUnauthorized)
			return
		}

		token := strings.TrimPrefix(authHeader, bearerPrefix)
		user, err := g.check(token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx := contextWithUser(r.Context(), user)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
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
