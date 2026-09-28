package remote

import "testing"

func TestCleanupChildProviderPrecedence(t *testing.T) {
	for _, tc := range []struct{ explicit, child, parent, want string }{
		{" gitlab ", "https://github.com/acme/repo/issues/2", "https://github.com/acme/repo/issues/1", "gitlab"},
		{" ", "https://github.com/acme/repo/issues/2", "https://gitlab.com/acme/repo/-/issues/1", "github"},
		{"", "invalid", "https://gitlab.com/acme/repo/-/issues/1", "gitlab"},
		{"", "invalid", "invalid", ""},
	} {
		if got := CleanupChildProvider(tc.explicit, tc.child, tc.parent); got != tc.want {
			t.Fatalf("provider(%+v)=%q", tc, got)
		}
	}
}
