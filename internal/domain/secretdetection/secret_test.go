package secretdetection

import "testing"

func TestContainsRejectsCredentialShapes(t *testing.T) {
	for _, value := range []string{
		"api_key=secret-value",
		"-----BEGIN OPENSSH PRIVATE KEY-----",
		"ghp_123456789012345678901234567890",
	} {
		if !Contains(value) {
			t.Fatalf("secret pattern was not detected: %q", value)
		}
	}
	if Contains("# Plan\nNo credentials here.\n") {
		t.Fatal("ordinary artifact text was rejected")
	}
}

func TestFindReportsEarliestMatchLineAndKindWithoutValue(t *testing.T) {
	// The token on line 4 comes later in the text than the assignment on
	// line 3, even though its pattern is checked later too.
	text := "# Plan\n\napi_key=hunter2\nghp_123456789012345678901234567890\n"
	if match, found := Find(text); !found || match != (Match{Line: 3, Kind: "credential assignment"}) {
		t.Fatalf("match=%+v found=%v", match, found)
	}
	if match, found := Find("intro\n" + "ghp_123456789012345678901234567890 then api_key=x"); !found || match != (Match{Line: 2, Kind: "GitHub/GitLab token"}) {
		t.Fatalf("earliest offset must win across patterns: %+v", match)
	}
	if _, found := Find("# Plan\nNo credentials here.\n"); found {
		t.Fatal("ordinary text matched")
	}
}
