package issueopsapp

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	core "issueops/internal/adapter/issueops"
	model "issueops/internal/contract/issueops"
)

func TestArtifactVerificationCompositionRechecksLatestAuthority(t *testing.T) {
	for _, mode := range []string{"invalid-phase", "live-failure", "ancestry-failure", "holder-transfer", "phase-change", "project-change", "success"} {
		t.Run(mode, func(t *testing.T) {
			root, repo, worktree := t.TempDir(), t.TempDir(), t.TempDir()
			record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: repo, Branch: "68-artifact"})
			if err != nil {
				t.Fatal(err)
			}
			process := liveFixtureReceipt(t)
			record.Phase = model.IssueOpsPhasePR
			record.IssueURL = "https://github.com/acme/repo/issues/68"
			record.Execution = &model.Execution{Mode: model.ExecutionModeDirect, Workspace: model.Workspace{SourceRoot: repo, Root: worktree, Branch: record.Branch, BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "then"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &process}, ClaimedAt: "then"}}
			if mode == "invalid-phase" {
				record.Phase = model.IssueOpsPhasePlan
			}
			if _, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			req := model.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pull_request", URL: "https://github.com/acme/repo/pull/68", TargetBranch: " main ", Labels: []string{" bug ", "bug"}, Assignees: []string{" owner "}}
			var events []string
			live := func(_ context.Context, got model.IssueOpsRemoteArtifactVerificationRequest) error {
				events = append(events, "live")
				if !reflect.DeepEqual(got, req) {
					t.Fatalf("live request changed: %+v", got)
				}
				if mode == "live-failure" {
					return errors.New("live unavailable")
				}
				latest, e := core.ReadIssueOps(root, record.ID)
				if e != nil {
					t.Fatal(e)
				}
				latest.AISlopCleanVerification = []string{"concurrent evidence"}
				switch mode {
				case "holder-transfer":
					latest.Execution.Lease.Holder.SessionID = "new-holder"
				case "phase-change":
					latest.Phase = model.IssueOpsPhasePlan
				case "project-change":
					latest.IssueURL = "https://github.com/other/repo/issues/68"
				}
				if _, e = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), latest); e != nil {
					t.Fatal(e)
				}
				return nil
			}
			observe := func() ([]model.NativeProcessReceipt, error) {
				events = append(events, "ancestry")
				if mode == "ancestry-failure" {
					return nil, errors.New("ancestry unavailable")
				}
				return []model.NativeProcessReceipt{process}, nil
			}
			now := time.Date(2026, 9, 28, 3, 0, 0, 0, time.UTC)
			service := newArtifactVerificationService(root, live, observe, func() time.Time { return now })
			got, err := service.Verify(context.Background(), record.ID, req, model.IssueOpsActor{Host: "codex", SessionID: "holder", CWD: worktree})
			persisted, readErr := core.ReadIssueOps(root, record.ID)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if mode == "success" {
				if err != nil {
					t.Fatal(err)
				}
				artifact := got.RemoteArtifact
				if artifact == nil || artifact.Kind != "pr" || artifact.TargetBranch != "main" || strings.Join(artifact.Labels, ",") != "bug" || strings.Join(artifact.Assignees, ",") != "owner" || artifact.VerifiedAt != now.Format(time.RFC3339Nano) || got.UpdatedAt != artifact.VerifiedAt {
					t.Fatalf("record=%+v", got)
				}
				if !reflect.DeepEqual(persisted.RemoteArtifact, artifact) || strings.Join(persisted.AISlopCleanVerification, ",") != "concurrent evidence" {
					t.Fatalf("lost update: %+v", persisted)
				}
			} else if err == nil || got.OK || persisted.RemoteArtifact != nil {
				t.Fatalf("invalid verification persisted: err=%v got=%+v stored=%+v", err, got, persisted)
			}
			want := []string{"live", "ancestry"}
			if mode == "invalid-phase" {
				want = nil
			}
			if mode == "live-failure" {
				want = []string{"live"}
			}
			if !reflect.DeepEqual(events, want) {
				t.Fatalf("events=%v want=%v", events, want)
			}
		})
	}
}
