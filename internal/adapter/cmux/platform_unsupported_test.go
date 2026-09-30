//go:build !darwin && !linux

package cmux

import (
	"context"
	"strings"
	"testing"

	cmuxcontract "issueops/internal/contract/cmux"
)

func TestUnsupportedPlatformFailsClosed(t *testing.T) {
	assertUnsupported := func(name string, err error) {
		t.Helper()
		if err == nil || !strings.Contains(err.Error(), "unsupported") {
			t.Fatalf("%s error=%v", name, err)
		}
	}
	_, err := (Client{}).Preflight(context.Background(), cmuxcontract.PreflightRequest{})
	assertUnsupported("preflight", err)
	_, err = (Client{}).CreateWorkspace(context.Background(), cmuxcontract.CreateRequest{})
	assertUnsupported("create", err)
	_, err = (Client{}).Send(context.Background(), cmuxcontract.SendRequest{})
	assertUnsupported("send", err)
	_, err = ReadPrompt("/worktree", "/worktree/prompt", strings.Repeat("a", 64))
	assertUnsupported("prompt", err)
	_, err = prepareTestLauncher(cmuxcontract.ArtifactRequest{})
	assertUnsupported("launcher", err)
	_, err = ObserveEndpoint("/tmp/cmux.sock", 0)
	assertUnsupported("endpoint", err)
}
