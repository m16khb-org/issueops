package policy

import (
	"reflect"
	"strings"
	"testing"
)

func TestBoundedDiagnosticRedactsAndCapsExternalText(t *testing.T) {
	if got := BoundedDiagnostic("token=private-value", 32); got != "<redacted>" {
		t.Fatalf("redacted diagnostic = %q", got)
	}
	got := BoundedDiagnostic(strings.Repeat("x", 64), 16)
	if got != strings.Repeat("x", 16)+"...[truncated]" {
		t.Fatalf("bounded diagnostic = %q", got)
	}
}

func TestRedactFreeformRedactsKnownBareTokenFormats(t *testing.T) {
	values := []string{
		"gh" + "p_abcdefghijklmnopqrstuvwxyz123456",
		"github_" + "pat_abcdefghijklmnopqrstuvwxyz123456",
		"xox" + "b-123456789012-abcdefghijklmnop",
		"sk" + "-abcdefghijklmnopqrstuvwxyz123456",
		"AK" + "IAABCDEFGHIJKLMNOP",
	}
	for _, value := range values {
		if got := RedactFreeform(value); got != "<redacted>" {
			t.Fatalf("RedactFreeform(%q) = %q", value, got)
		}
	}
}

// CleanEnvAllowlist/ValidEnvName/RedactArgv는 감사 로그 env allowlist와 argv
// redaction의 진입 규칙이다.
func TestCleanEnvAllowlistTrimsSortsDedupesNothing(t *testing.T) {
	got := CleanEnvAllowlist([]string{" PATH ", "", "HOME", "PATH", "  "})
	want := []string{"HOME", "PATH", "PATH"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowlist = %v want %v", got, want)
	}
	if got := CleanEnvAllowlist(nil); len(got) != 0 {
		t.Fatalf("nil allowlist must be empty: %v", got)
	}
}

func TestValidEnvName(t *testing.T) {
	for _, ok := range []string{"PATH", "HOME", "_FOO", "a", "A_1_b"} {
		if !ValidEnvName(ok) {
			t.Fatalf("%q must be a valid env name", ok)
		}
	}
	for _, bad := range []string{"", "1PATH", "-FOO", "FO O", "FOO-BAR", "한글"} {
		if ValidEnvName(bad) {
			t.Fatalf("%q must be invalid", bad)
		}
	}
}

func TestRedactArgvRedactsOnlySecretArgs(t *testing.T) {
	argv := []string{"git", "push", "--token=abc123", "/repo/credentials.json", "origin"}
	got := RedactArgv(argv)
	if got[0] != "git" || got[2] != "<redacted>" || got[3] != "<redacted>" || got[4] != "origin" {
		t.Fatalf("argv redaction wrong: %v", got)
	}
	if got := RedactArgv(nil); len(got) != 0 {
		t.Fatalf("nil argv must produce an empty slice: %v", got)
	}
}

// The keyword prefilter must never hide a match: every input that the
// regexes flag also passes mayHoldSecret, and SecretLikeBytes agrees with
// SecretLikeArg on all of them.
func TestSecretKeywordsCoverEveryPattern(t *testing.T) {
	flagged := []string{
		"TOKEN=x", "Password=x", "passwd=x", "Secret=x", "API_KEY=x", "apikey=x", "api-key=x", "credential=x",
		"Authorization: Bearer abc", "authorization=x",
		"ghp_" + strings.Repeat("a", 20), "GHO_" + strings.Repeat("a", 20), "ghu_" + strings.Repeat("a", 20),
		"ghs_" + strings.Repeat("a", 20), "ghr_" + strings.Repeat("a", 20), "github_pat_" + strings.Repeat("a", 20),
		"xoxb-" + strings.Repeat("a", 20), "sk-" + strings.Repeat("a", 20), "AKIA" + strings.Repeat("A", 16),
		".env", "a/.env.local", "id_rsa", "x/id_dsa", "id_ecdsa", "id_ed25519", "cert.pem", "a.KEY", "b.p12", "c.pfx",
		"aws_credentials", "my-secret-file", "x/\u212Aey.pem",
	}
	for _, value := range flagged {
		if !SecretLikeArg(value) {
			t.Fatalf("fixture %q is not secret-like", value)
		}
		if !mayHoldSecret([]byte(value)) || !SecretLikeBytes([]byte(value)) {
			t.Errorf("prefilter hides %q", value)
		}
		long := []byte(strings.Repeat("ok line\n", 1000) + value)
		if SecretLikeBytes(long) != SecretLikeArg(string(long)) {
			t.Errorf("prefilter changes the verdict for %q after 8 KB", value)
		}
	}
	for _, value := range []string{"ok  issueops/internal 0.2s\n", strings.Repeat("PASS\n", 5000)} {
		if mayHoldSecret([]byte(value)) || SecretLikeBytes([]byte(value)) {
			t.Errorf("plain output %q flagged", value[:min(20, len(value))])
		}
	}
}
