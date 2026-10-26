package providerutil

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestDryRunPreviewUTF8Safe(t *testing.T) {
	got := DryRunPreview("gh", "issue", "create", "--title", strings.Repeat("가", 2000))
	if !utf8.ValidString(got) {
		t.Fatalf("preview is not valid UTF-8: %q", got[len(got)-32:])
	}
	if !strings.HasSuffix(got, "...[truncated]") {
		t.Fatalf("preview lost the truncation suffix: %q", got[len(got)-32:])
	}
	if limit := len("[dry-run] would execute: ") + providerDiagnosticLimit + len("...[truncated]"); len(got) > limit {
		t.Fatalf("preview length = %d, want <= %d", len(got), limit)
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `�`) {
		t.Fatalf("JSON preview contains a replacement character")
	}
}

func TestBoundedCommandStderrUTF8Safe(t *testing.T) {
	// 1366 characters are 4098 bytes, so the 4096-byte stderr limit lands one
	// byte into the last kept character.
	_, _, err := RunBoundedMutationContext(context.Background(), t.TempDir(), "sh", "-c", "printf '%s' \"$0\" >&2; exit 1", strings.Repeat("가", 1366))
	if err == nil {
		t.Fatal("expected the command to fail")
	}
	if !utf8.ValidString(err.Error()) {
		t.Fatalf("diagnostic is not valid UTF-8")
	}
	encoded, marshalErr := json.Marshal(err.Error())
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(encoded), `�`) {
		t.Fatalf("JSON diagnostic contains a replacement character")
	}
}

// The body value is shown as its length; the shell-quoted copy repeated the
// file the caller wrote in every preview. A stdin body ("-") stays literal.
func TestDryRunPreviewShowsBodyLength(t *testing.T) {
	got := DryRunPreview("gh", "pr", "create", "--title", "t", "--body", "## 요약\n\nit's done")
	if got != "[dry-run] would execute: gh pr create --title t --body '<body: 16 chars>'" {
		t.Fatalf("preview = %q", got)
	}
	if got := DryRunPreview("gh", "pr", "edit", "u", "--body-file", "-"); got != "[dry-run] would execute: gh pr edit u --body-file -" {
		t.Fatalf("stdin preview = %q", got)
	}
}
