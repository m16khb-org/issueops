package cmux

import (
	"errors"
	"io/fs"
	"testing"
)

func TestMutationErrorMessageAndUnwrap(t *testing.T) {
	cause := fs.ErrPermission
	err := error(&MutationError{Phase: "send", Ambiguous: true, Cause: cause})
	if got, want := err.Error(), "cmux send failed: permission denied"; got != want {
		t.Fatalf("message = %q, want %q", got, want)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("cause not reachable via errors.Is")
	}
}

func TestMutationErrorNilReceiverIsEmpty(t *testing.T) {
	var err *MutationError
	if got := err.Error(); got != "" {
		t.Fatalf("nil receiver message = %q", got)
	}
}
