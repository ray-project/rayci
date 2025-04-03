package reefd

import "context"

type contextKey int // userKey is the key used to store the user in the context.

const (
	userKey contextKey = iota
)

func contextWithUser(ctx context.Context, user string) context.Context {
	return context.WithValue(ctx, userKey, user)
}

func userFromContext(ctx context.Context) (string, bool) {
	user, ok := ctx.Value(userKey).(string)
	return user, ok
}
