package mcpcli

import (
	"context"
	"testing"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	commitapp "issueops/internal/application/commitsuggest"
	lintapp "issueops/internal/application/lintdiagnose"
	"issueops/internal/testsupport"
)

type callerContextKey struct{}

// callerContextEffects records whether the context its command effect receives
// carries the caller's marker.
type callerContextEffects struct{ got *any }

func (callerContextEffects) NormalizeRoot(root string) (string, error) { return root, nil }
func (e callerContextEffects) Diff(ctx context.Context, _ string, _ bool) (string, error) {
	*e.got = ctx.Value(callerContextKey{})
	return "", nil
}
func (e callerContextEffects) Run(ctx context.Context, _ string, _ []string) (string, int, bool, error) {
	*e.got = ctx.Value(callerContextKey{})
	return "", 0, false, nil
}

// The MCP request context reaches the command effect of commit_suggest and
// lint_diagnose, so cancelling the request can stop the child process.
func TestAssistantToolsPassCallerContext(t *testing.T) {
	const marker = "caller-request"
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{name: "commit_suggest", args: map[string]any{}},
		{name: "lint_diagnose", args: map[string]any{"command_argv": []any{"lint"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got any
			effects := callerContextEffects{got: &got}
			deps := MCPDependencies{
				Catalog:       mcpcatalog.Build(),
				DefaultTarget: testsupport.AbsPath("repo"),
				Commit:        commitapp.Service{Effects: effects},
				Lint:          lintapp.Service{Effects: effects},
			}
			ctx := context.WithValue(t.Context(), callerContextKey{}, marker)
			outcome := dispatchMCPToolCall(ctx, MCPToolCall{Name: tc.name, Arguments: tc.args}, deps, transportStdio)
			if !outcome.Handled || outcome.IsError || outcome.Err != nil {
				t.Fatalf("%s outcome = %#v", tc.name, outcome)
			}
			if got != marker {
				t.Fatalf("%s effect got context value %v, want the caller's %q", tc.name, got, marker)
			}
		})
	}
}
