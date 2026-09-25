package remoteverify

import (
	"os/exec"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRemoteVerifyStderrUTF8Safe(t *testing.T) {
	// 700 characters are 2100 bytes, so the 2048-byte stderr limit lands two
	// bytes into the last kept character.
	_, err := executeBoundedRemoteVerifyCommand(exec.Command("sh", "-c", "printf '%s' \"$0\" >&2; exit 1", strings.Repeat("가", 700)))
	if err == nil {
		t.Fatal("expected the command to fail")
	}
	if !utf8.ValidString(err.Error()) {
		t.Fatalf("diagnostic is not valid UTF-8")
	}
}

func TestCommandOutputErrorUTF8Safe(t *testing.T) {
	err := commandOutputError(&exec.ExitError{Stderr: []byte(strings.Repeat("가", 700))})
	if !utf8.ValidString(err.Error()) {
		t.Fatalf("diagnostic is not valid UTF-8")
	}
	if len(err.Error()) > maxRemoteVerifyDiagnosticBytes {
		t.Fatalf("diagnostic length = %d", len(err.Error()))
	}
}
