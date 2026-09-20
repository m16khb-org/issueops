//go:build !darwin && !linux

package cmux

import (
	"context"
	"strings"
	"testing"
)

func TestUnsupportedPlatformFailsClosed(t *testing.T) {
	assertUnsupported := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("%s error=%v", name, err)
		}
	}
	_, err := (Client{}).Preflight(context.Background(), PreflightRequest{})
	assertUnsupported("preflight", err)
	_, err = (Client{}).CreateWorkspace(context.Background(), CreateRequest{})
	assertUnsupported("create", err)
	_, err = (Client{}).Send(context.Background(), SendRequest{})
	assertUnsupported("send", err)
	_, err = ReadPrompt("/worktree", "/worktree/prompt", strings.Repeat("a", 64))
	assertUnsupported("prompt", err)
	_, err = PrepareLauncher(ArtifactRequest{})
	assertUnsupported("launcher", err)
	_, err = ObserveEndpoint("/tmp/cmux.sock", 0)
	assertUnsupported("endpoint", err)
}
