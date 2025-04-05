package reefd

import (
	"context"
	"testing"
)

func TestContextWithUser(t *testing.T) {
	ctx := context.Background()
	empty, ok := userFromContext(ctx)
	if ok {
		t.Errorf("got ok %v, want false", ok)
	}
	if empty != "" {
		t.Errorf("got user %q, want empty", empty)
	}

	ctx = contextWithUser(ctx, "testuser")
	got, ok := userFromContext(ctx)
	if !ok {
		t.Errorf("got ok %v, want true", ok)
	}
	if got != "testuser" {
		t.Errorf("got user %q, want %q", got, "testuser")
	}
}
