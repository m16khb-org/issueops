package issueops

import (
	"context"
	"testing"
)

// A gh/glab issue create whose body carries the operation ID, or issueops
// remote create-issue for the cycle, is in flight; other commands are not.
// The match uses ASCII identities a C-locale ps keeps (titles may be Korean).
func TestIssueCreateInFlight(t *testing.T) {
	const op = "ea7db9862c1470fbc2cc3657d93db0c4"
	for _, tc := range []struct {
		rows string
		want bool
	}{
		{"/opt/homebrew/bin/gh issue create --repo o/r --title ??? --body text <!-- issueops:issue-create:" + op + " -->", true},
		{"bash /home/u/.shims/glab issue create --title x --description <!-- issueops:issue-create:" + op + " -->", true},
		{"/usr/local/bin/issueops remote create-issue --id io-1 --title x --confirm", true},
		{"/opt/homebrew/bin/gh issue create --title other --body <!-- issueops:issue-create:ffffffffffffffffffffffffffffffff -->", false},
		{"/opt/homebrew/bin/gh issue list --search " + op, false},
		{"glab api graphql -f query=mutation workItemCreate { workItemCreate(input: { description: \"<!-- issueops:issue-create:" + op + " -->\" }) }", true},
		{"glab api graphql -f query=query childRecoverySearch($marker: String!) {...} -f marker=" + op, false},
		{"/usr/local/bin/issueops remote create-child --id=io-1 --title x --confirm", true},
		{"/usr/local/bin/issueops remote reconcile-issue --id io-1 --confirm", false},
	} {
		if got := issueCreateInFlight(tc.rows, op, "io-1"); got != tc.want {
			t.Fatalf("%q: got %v", tc.rows, got)
		}
	}
}

// One create of an issue at a time: the second is refused before invoking.
func TestHoldIssueCreateRefusesASecondCreate(t *testing.T) {
	root := t.TempDir()
	_, release, err := HoldIssueCreate(context.Background(), root, "/repo", "Add dec function")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, _, err := HoldIssueCreate(context.Background(), root, "/repo", "Add dec function"); err == nil {
		t.Fatal("second create was not refused")
	}
	if _, other, err := HoldIssueCreate(context.Background(), root, "/repo", "Another issue"); err != nil {
		t.Fatalf("another title must not be blocked: %v", err)
	} else {
		other()
	}
}
