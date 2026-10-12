package issueopspreparation

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func TestPreparePreviewConfirmCommandOmitsUnsetOptionalFlags(t *testing.T) {
	fixture := newDirectServiceFixture()
	command := directCommand(false)
	command.CWD = ""
	result, err := fixture.service.Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"--owner-host", "--owner-model", "--owner-effort", "--agent-id", "--cwd"} {
		if strings.Contains(result.NextCommand, flag) {
			t.Fatalf("unset value must not produce %s: %s", flag, result.NextCommand)
		}
	}
}

func TestPreparePreviewConfirmCommandCarriesSetHostAndCWD(t *testing.T) {
	fixture := newDirectServiceFixture()
	result, err := fixture.service.Prepare(context.Background(), directCommand(false))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--host 'codex'", "--cwd '/repo'"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("confirm command %q lacks %q", result.NextCommand, want)
		}
	}
	if strings.Contains(result.NextCommand, "--owner-") {
		t.Fatalf("direct preview without owner profile must not carry owner flags: %s", result.NextCommand)
	}
}

func TestPrepareOrcaPreviewConfirmCommandCarriesOwnerProfile(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeOrca))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"--owner-host 'codex'", "--owner-model 'gpt-6.1-sol'", "--owner-effort 'high'"} {
		if !strings.Contains(result.NextCommand, want) {
			t.Fatalf("confirm command %q lacks %q", result.NextCommand, want)
		}
	}
}

func TestPrepareProbeErrorKeepsReportedProbeCodeForAutoFallback(t *testing.T) {
	fixture := newOrcaApplicationFixture()
	fixture.gateway.probe = preparationcontract.ProbeResult{Code: "orca_runtime_busy"}
	fixture.gateway.probeErr = errors.New("socket closed")
	result, err := fixture.service.Prepare(context.Background(), orcaCommand(false, preparationcontract.ModeAuto))
	if err != nil {
		t.Fatal(err)
	}
	if result.ProbeCode != "orca_runtime_busy" {
		t.Fatalf("a code reported by the probe must survive a transport error: %+v", result)
	}
}

func TestWriterlessErrorForClaimableOrcaDependsOnBindingGeneration(t *testing.T) {
	for _, tc := range []struct {
		name       string
		binding    *leasecontract.OrcaBinding
		want       string
		wantAbsent string
	}{
		{"binding matches lease generation", &leasecontract.OrcaBinding{LeaseGeneration: 1}, "generation 1 is claimable and has no writer", "no current owner"},
		{"binding from older generation", &leasecontract.OrcaBinding{LeaseGeneration: 2}, "Orca generation 1 has no current owner", "has no writer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			execution := applicationExecution("orca", "claimable", false)
			execution.Orca = tc.binding
			err := writerlessError(leasecontract.Record{ID: "io-1", Execution: execution}, "NEXT")
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), tc.wantAbsent) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

type aliasPathFiles struct {
	*intentFilesFake
}

// SamePath treats /alias as the same directory as /parent and never matches an empty path,
// like a filesystem comparison that cannot resolve nothing.
func (f aliasPathFiles) SamePath(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	canonical := func(p string) string {
		if p == "/alias" {
			return "/parent"
		}
		return p
	}
	return canonical(a) == canonical(b)
}

func TestIntentRequestBuilderInspectComparesSealedParentWorktreeThroughPathEquivalence(t *testing.T) {
	for _, tc := range []struct {
		name             string
		recorded, sealed string
		wantErr          bool
	}{
		{"both unset", "", "", false},
		{"equivalent spellings of one parent", "/parent", "/alias", false},
		{"different parents", "/parent", "/other", true},
		{"recorded parent missing from seal", "/parent", "", true},
		{"sealed parent missing from record", "", "/parent", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := leasecontract.Record{
				ID: "io-1", IssueURL: "https://github.com/acme/repo/issues/21",
				BranchPrepare: json.RawMessage(`{"provider":"github","issue_url":"https://github.com/acme/repo/issues/21","link_verified":true}`),
			}
			begin := OrcaBegin{
				Workspace:   preparationcontract.WorkspaceRequest{LifecycleID: record.ID, SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base"},
				Command:     preparationcontract.Command{OwnerHost: "codex", OwnerModel: "model", OwnerEffort: "high"},
				Probe:       preparationcontract.ProbeRequest{Repo: "/source", Host: "codex", Model: "model", Effort: "high", Provider: "github", Issue: 21},
				Owner:       preparationcontract.OwnerEvidence{Provider: "github", Issue: 21, BodySHA256: strings.Repeat("a", 64)},
				OperationID: "0123456789abcdef0123456789abcdef", StartedAt: "2026-08-02T00:00:00Z",
			}
			recorded, intent, err := ApplyOrcaBegin(record, begin, "artifact-dir")
			if err != nil {
				t.Fatal(err)
			}
			recorded.Execution.Workspace.ParentWorktree = tc.recorded
			intent.Workspace.ParentWorktree = tc.sealed
			builder := IntentRequestBuilder{Files: aliasPathFiles{&intentFilesFake{}}}
			_, err = builder.Inspect(recorded, intent)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
		})
	}
}
