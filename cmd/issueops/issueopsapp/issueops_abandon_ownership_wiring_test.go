package issueopsapp

import (
	"context"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"issueops/cmd/issueops/issueopscli/feedbackcleanup"
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	github "issueops/internal/adapter/provider/github"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type abandonWiringProvider struct {
	github.Provider
	read func(port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error)
}

func (p abandonWiringProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	return p.read(req)
}

func TestAbandonCLIObservesOnlyAfterOwnershipAndBindsLoadedArtifact(t *testing.T) {
	for _, mode := range []string{"live", "foreign", "artifact drift"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			t.Setenv("ISSUEOPS_STATE_DIR", root)
			root = core.IssueOpsStateRoot()
			configureIssueOpsCleanup()
			record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: makeGitRepoForContract(t), Branch: "991-abandon-wiring"})
			if err != nil {
				t.Fatal(err)
			}
			record.RemoteArtifact = &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1"}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			save := func(r model.IssueOpsRecord) {
				raw, err := json.Marshal(r)
				if err != nil {
					t.Fatal(err)
				}
				if err := db.Put("issueops_v1", r.ID, raw); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "foreign" {
				record.CleanupAttempt = &model.IssueOpsCleanupAttempt{Operation: model.CleanupOperationFinish, Token: strings.Repeat("a", 64), StartedAt: "2026-09-29T00:00:00Z"}
			}
			save(record)
			if mode == "live" {
				lease, err := (core.CleanupLifetimeLock{StateRoot: root}).Acquire(context.Background(), record.ID)
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
			}
			providers, reads, effects := 0, 0, 0
			drift := false
			var output model.CleanupAbandonResult
			provider := abandonWiringProvider{read: func(req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
				reads++
				if req.URL != record.RemoteArtifact.URL {
					t.Fatalf("wrong loaded artifact: %s", req.URL)
				}
				if drift {
					replacement := record
					artifact := *record.RemoteArtifact
					artifact.URL = "https://github.com/acme/repo/pull/2"
					replacement.RemoteArtifact = &artifact
					save(replacement)
				}
				return port.IssueProviderArtifactBody{State: "OPEN"}, nil
			}}
			deps := feedbackcleanup.Deps{
				ParseFlags: func(fs *flag.FlagSet, args []string) (bool, error) { return false, fs.Parse(args) },
				PrintJSON: func(v any) error {
					if r, ok := v.(model.CleanupAbandonResult); ok {
						output = r
					}
					return nil
				}, PrintError: func(error) error { return nil },
				Provider: func(string) (port.IssueProvider, error) { providers++; return provider, nil },
				ObserveArtifactMerged: func(model.IssueOpsRemoteArtifactVerification) (bool, error) {
					t.Error("legacy transport observation called")
					return false, nil
				},
				CleanupFinishGit: func(_ string, args ...string) (int, string) {
					if args[0] == "rev-parse" {
						return 1, ""
					}
					effects++
					return 0, ""
				},
			}
			args := []string{"abandon", "--id", record.ID, "--reason", "ownership wiring", "--preview", "--json"}
			err = feedbackcleanup.RunCleanup(args, deps)
			if mode != "artifact drift" {
				if err == nil || providers != 0 || reads != 0 || effects != 0 {
					t.Fatalf("ownership refusal observed externally: providers=%d reads=%d effects=%d err=%v", providers, reads, effects, err)
				}
				return
			}
			if err != nil || output.Fingerprint == "" || providers != 1 || reads != 1 {
				t.Fatalf("preview failed: %+v %v", output, err)
			}
			drift = true
			args = []string{"abandon", "--id", record.ID, "--reason", "ownership wiring", "--apply", "--confirm", "--fingerprint", output.Fingerprint, "--json"}
			if err := feedbackcleanup.RunCleanup(args, deps); err == nil {
				t.Fatal("artifact replacement accepted")
			}
			kept, err := core.ReadIssueOps(root, record.ID)
			if err != nil || kept.RemoteArtifact.URL != "https://github.com/acme/repo/pull/2" || kept.CleanupAttempt != nil || effects != 0 {
				t.Fatalf("stale evidence mutated replacement: %+v effects=%d err=%v", kept, effects, err)
			}
		})
	}
}
