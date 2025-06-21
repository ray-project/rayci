package reefapi

// TokenRequest is the encoded request to sign in with a user name.
type TokenRequest struct {
	// User is the user name to sign in with.
	User string `json:"user"`
}

// SSHSignature is the signature structure of the verifying an token request
// that is signed with an SSH key.
type SSHSignature struct {
	Format string `json:"format"`
	Blob   []byte `json:"blob"`
	Rest   []byte `json:"rest,omitempty"`
}

// LoginRequest is the request to sign in as an user.
type LoginRequest struct {
	// User is the user name to sign in with.
	// It hints on which user and key to use to verify the token request.
	User string `json:"user"`

	// TokenRequest is the JSON encoded token request.
	TokenRequest []byte `json:"token_request"`

	// SigningMethod is the method used to sign the token request.
	SigningMethod string `json:"signing_method"`

	// Signature is the cryptographic signature of the token request.
	Signature []byte `json:"signature"`
}

// LoginResponse is the response to a successful login request.
// It contains the session token to use for subsequent requests.
type LoginResponse struct {
	SessionToken string `json:"session_token"`
}

// LogoutRequest is the request to log out of a session.
type LogoutRequest struct {
	// SessionToken is the session token to log out.
	SessionToken string `json:"session_token"`
}

// LogoutResponse is the response to a successful logout request.
type LogoutResponse struct{}
