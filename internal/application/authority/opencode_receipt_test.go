package authority

import (
	"context"
	"fmt"
	"strings"
	"testing"

	contract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

// argumentsTable is a process table that also reports argv. Its own start
// identity lets a test model a PID reused between Inspect and Arguments.
type argumentsTable struct {
	*processTable
	args map[int]struct {
		startedAt string
		fields    []string
	}
}

func (a *argumentsTable) Arguments(_ context.Context, receipt model.NativeProcessReceipt) ([]string, error) {
	entry, ok := a.args[receipt.PID]
	if !ok {
		return nil, fmt.Errorf("pid %d is not live", receipt.PID)
	}
	if entry.startedAt != receipt.StartedAt {
		return nil, fmt.Errorf("pid %d start identity changed", receipt.PID)
	}
	return entry.fields, nil
}

func TestVerifyNativeRequiresStandaloneOpencodeServer(t *testing.T) {
	const opencodeBin = "/opt/homebrew/Cellar/opencode-v2/2.0.32/bin/opencode"
	for _, tc := range []struct {
		name          string
		host          string
		args          string
		argsStartedAt string
		noArguments   bool
		wantErr       string
	}{
		{name: "standalone server", host: "opencode", args: opencodeBin + " serve --stdio --port 0"},
		{name: "shared service with live tui parent", host: "opencode", args: opencodeBin + " serve --service", wantErr: "requires a --standalone session"},
		{name: "declared codex host on shared service", host: "codex", args: opencodeBin + " serve --service", wantErr: "requires a --standalone session"},
		{name: "tui prompt mentions the server", host: "opencode", args: opencodeBin + ` --prompt "x opencode serve --stdio"`, wantErr: "requires a --standalone session"},
		{name: "no argument inspector", host: "opencode", args: opencodeBin + " serve --stdio --port 0", noArguments: true, wantErr: "requires a --standalone session"},
		{name: "start identity mismatch", host: "opencode", args: opencodeBin + " serve --stdio --port 0", argsStartedAt: "2026-10-18T09:00:01Z", wantErr: "requires a --standalone session"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			receipt := model.NativeProcessReceipt{PID: 7070, StartedAt: "2026-10-18T09:00:00Z", Executable: opencodeBin}
			procs := &processTable{live: map[int]model.NativeProcessReceipt{receipt.PID: receipt}}
			argsStartedAt := receipt.StartedAt
			if tc.argsStartedAt != "" {
				argsStartedAt = tc.argsStartedAt
			}
			var service *Service
			grants := &memoryGrants{records: map[string][]byte{}}
			files := &memoryFiles{files: map[string]string{}}
			if tc.noArguments {
				service = New(grants, procs, files, newHarness().clock.Now, scopes{"/repo": repoScope}, &counterEntropy{})
			} else {
				inspector := &argumentsTable{processTable: procs, args: map[int]struct {
					startedAt string
					fields    []string
				}{receipt.PID: {startedAt: argsStartedAt, fields: strings.Fields(tc.args)}}}
				service = New(grants, inspector, files, newHarness().clock.Now, scopes{"/repo": repoScope}, &counterEntropy{})
			}
			actor := model.NativeActor{
				Host: tc.host, SessionID: "ses_1", SessionProcess: &receipt,
				ProcessAncestry: []model.NativeProcessReceipt{receipt, {PID: 1, StartedAt: "init", Executable: "/sbin/launchd"}},
			}
			_, verifyErr := service.Verify(context.Background(), actor)
			_, issueErr := service.Issue(context.Background(), contract.IssueRequest{WorkspaceRoot: "/repo", Actor: actor})
			for label, err := range map[string]error{"verify": verifyErr, "issue": issueErr} {
				if tc.wantErr == "" {
					if err != nil {
						t.Fatalf("%s rejected standalone server: %v", label, err)
					}
					continue
				}
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("%s err = %v, want containing %q", label, err, tc.wantErr)
				}
			}
			if tc.wantErr != "" && len(grants.records) != 0 {
				t.Fatalf("rejected receipt persisted %d grants", len(grants.records))
			}
		})
	}
}
