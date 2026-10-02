package issueopsapp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/port"
)

type bodySyncWiringProvider struct {
	publicationProviderFake
	body, receipt string
	updated       bool
	writes        int
}

func (p *bodySyncWiringProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	return port.IssueProviderArtifactBody{Provider: "github", Kind: req.Kind, URL: req.URL, Body: p.body, State: "OPEN"}, nil
}

func (p *bodySyncWiringProvider) ReplaceArtifactBody(_ context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	if !req.Confirm {
		return port.IssueProviderReplaceArtifactBodyResult{OK: true, Preview: "preview"}, nil
	}
	p.writes++
	p.body = req.Body
	digest := sha256.Sum256([]byte(strings.TrimRight(req.Body, " \t\n")))
	receipt := hex.EncodeToString(digest[:])
	if p.receipt != "" {
		receipt = p.receipt
	}
	return port.IssueProviderReplaceArtifactBodyResult{OK: true, Updated: p.updated, VerifiedBodySHA256: receipt}, nil
}

func TestBodySyncCompositionPersistsOnlyVerifiedReadback(t *testing.T) {
	for _, scenario := range []string{"verified", "mismatched receipt", "not applied"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "412-body-sync"})
			if err != nil {
				t.Fatal(err)
			}
			record.IssueURL = "https://github.com/acme/repo/issues/412"
			record, err = issueops.WriteIssueOps(context.Background(), root, record)
			if err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			before, exists, err := db.Get("issueops_v1", record.ID)
			if err != nil || !exists {
				t.Fatalf("initial row: exists=%v err=%v", exists, err)
			}
			const block = "<!-- issueops:completion:start -->\nexisting completion\n<!-- issueops:completion:end -->"
			live := "old body\n\n" + block
			digest := sha256.Sum256([]byte(live))
			provider := &bodySyncWiringProvider{body: live, updated: scenario != "not applied"}
			if scenario == "mismatched receipt" {
				provider.receipt = strings.Repeat("a", 64)
			}
			_, result, err := syncIssueOpsRemoteArtifactBody(context.Background(), root, record.ID, contract.Command{
				Kind: contract.KindIssue, ProposedBody: readableWiringIssueBody, ExpectedBodySHA256: hex.EncodeToString(digest[:]), Confirm: true, AcceptRemoteEdits: true,
			}, provider, model.IssueOpsActor{})
			stored, readErr := issueops.ReadIssueOps(root, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if scenario != "verified" {
				wantError := "readback does not match"
				if scenario == "not applied" {
					wantError = "did not report"
				}
				after, exists, readErr := db.Get("issueops_v1", record.ID)
				if err == nil || !strings.Contains(err.Error(), wantError) || readErr != nil || !exists || !bytes.Equal(before, after) {
					t.Fatalf("unverified write changed storage: err=%v readErr=%v exists=%v baseline=%+v", err, readErr, exists, stored.BodySyncs)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !result.Updated || provider.writes != 1 || result.Drift != contract.DriftInSync || len(stored.BodySyncs) != 1 {
				t.Fatalf("sync=%+v writes=%d baseline=%+v", result, provider.writes, stored.BodySyncs)
			}
			if provider.body != readableWiringIssueBody+"\n\n"+block+"\n" {
				t.Fatalf("managed body changed: %q", provider.body)
			}
			written := sha256.Sum256([]byte(readableWiringIssueBody + "\n\n" + block))
			if stored.BodySyncs[0].ToSHA256 != hex.EncodeToString(written[:]) || stored.BodySyncs[0].URL != record.IssueURL || stored.BodySyncs[0].SyncedAt == "" {
				t.Fatalf("stored baseline=%+v", stored.BodySyncs[0])
			}
		})
	}
}
