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
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/ray-project/rayci/reefd/reefapi"
)

type authGate struct {
	sessionStore *sessionStore

	rand io.Reader

	nowFunc  func() time.Time
	userKeys map[string]string

	unauth map[string]struct{}
}

func newAuthGate(
	sessions *sessionStore, userKeys map[string]string,
	unauth []string,
) *authGate {
	unauthMap := make(map[string]struct{}, len(unauth))
	for _, u := range unauth {
		unauthMap[u] = struct{}{}
	}

	return &authGate{
		sessionStore: sessions,

		rand:     rand.Reader,
		nowFunc:  time.Now,
		userKeys: userKeys,
		unauth:   unauthMap,
	}
}

const sessionTokenPrefix = "ses_"

func (g *authGate) newSessionToken(
	ctx context.Context, req *reefapi.TokenRequest,
) (string, error) {
	const tokenSize = 24
	rand := make([]byte, tokenSize)
	if _, err := io.ReadFull(g.rand, rand); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}

	// Encode the token in base64 to make it URL safe.
	token := sessionTokenPrefix + base64.RawURLEncoding.EncodeToString(rand)
	now := g.nowFunc()
	const ttl = 10 * time.Hour

	session := &session{
		user:   req.User,
		token:  token,
		expire: now.Add(ttl),
	}

	if err := g.sessionStore.insert(ctx, session); err != nil {
		return "", fmt.Errorf("save session: %w", err)
	}

	return token, nil
}

func (g *authGate) apiLogin(ctx context.Context, req *reefapi.LoginRequest) (
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

	sessionToken, err := g.newSessionToken(ctx, tokenReq)
	if err != nil {
		return nil, fmt.Errorf("new session token: %w", err)
	}

	resp := &reefapi.LoginResponse{SessionToken: sessionToken}
	return resp, nil
}

func (g *authGate) check(ctx context.Context, token string) (string, error) {
	ses, err := g.sessionStore.get(ctx, token)
	if err != nil {
		return "", fmt.Errorf("get session: %w", err)
	}
	if g.nowFunc().After(ses.expire) {
		return "", fmt.Errorf("session %q has expired", token)
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
			http.Error(
				w, "authorization header is empty",
				http.StatusUnauthorized,
			)
			return
		}

		const bearerPrefix = "Bearer "

		if !strings.HasPrefix(authHeader, bearerPrefix) {
			http.Error(
				w, "authorization header must be Bearer",
				http.StatusUnauthorized,
			)
			return
		}

		ctx := r.Context()
		token := strings.TrimPrefix(authHeader, bearerPrefix)
		user, err := g.check(ctx, token)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}

		ctx = contextWithUser(ctx, user)
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (g *authGate) apiLogout(ctx context.Context, req *reefapi.LogoutRequest) (
	*reefapi.LogoutResponse, error,
) {
	if req.SessionToken == "" {
		return nil, fmt.Errorf("session token is empty")
	}
	if err := g.sessionStore.delete(ctx, req.SessionToken); err != nil {
		return nil, fmt.Errorf("delete session: %w", err)
	}
	return &reefapi.LogoutResponse{}, nil
}
