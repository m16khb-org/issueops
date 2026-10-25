package preflight

import "testing"

// message_policy_source says whether the policy path is the repository's own
// or the issueops default a repository without one falls back to.
func TestCommitStyleHintsNamesThePolicySource(t *testing.T) {
	for _, tc := range []struct{ path, root, want string }{
		{"/w/app/.issueops/COMMIT_POLICY.md", "/w/app", "repository"},
		{"/h/issueops/.issueops/COMMIT_POLICY.md", "/w/app", "issueops_default"},
		{"/w/app2/.issueops/COMMIT_POLICY.md", "/w/app", "issueops_default"},
	} {
		if got := CommitStyleHints(nil, nil, tc.path, tc.root)["message_policy_source"]; got != tc.want {
			t.Fatalf("%s in %s: %v", tc.path, tc.root, got)
		}
	}
}
