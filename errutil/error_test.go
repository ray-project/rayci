package errutil

import (
	"testing"

	"errors"
)

var errInside = errors.New("inside error")

func TestWrap(t *testing.T) {
	err := Wrap(errInside, "wrapping")
	if !errors.Is(err, errInside) {
		t.Errorf("wrapped error %q is not a %q", err, errInside)
	}

	wrappedNil := Wrap(nil, "wrapping")
	if wrappedNil == nil {
		t.Errorf("wrapped nil error should not be nil")
	}

	wrappedEmpty := Wrap(nil, "")
	if wrappedEmpty == nil {
		t.Errorf("wrapped empty error should not be nil")
	} else if wrappedEmpty.Error() == "" {
		t.Errorf("wrapped empty error should not have an empty message")
	}
}

func TestWrapf(t *testing.T) {
	err := Wrapf(errInside, "wrapper %d", 42)
	if !errors.Is(err, errInside) {
		t.Errorf("wrapped error %q is not a %q", err, errInside)
	}

	wrappedNil := Wrapf(nil, "wrapper %d", 42)
	if wrappedNil == nil {
		t.Errorf("wrapped nil error should not be nil")
	}

	wrappedEmpty := Wrap(nil, "")
	if wrappedEmpty == nil {
		t.Errorf("wrapped empty error should not be nil")
	} else if wrappedEmpty.Error() == "" {
		t.Errorf("wrapped empty error should not have an empty message")
	}
}
