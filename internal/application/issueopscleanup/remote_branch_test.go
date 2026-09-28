package issueopscleanup

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

type remoteBranchTestStore struct{ record model.IssueOpsRecord }

func (s *remoteBranchTestStore) Load(context.Context, string) (model.CleanupSnapshot, error) {
	return model.CleanupSnapshot{Record: s.record}, nil
}
func (s *remoteBranchTestStore) Arm(_ context.Context, snap model.CleanupSnapshot, a model.IssueOpsCleanupAttempt) (model.CleanupSnapshot, error) {
	snap.Record.CleanupAttempt = &a
	return snap, nil
}
func (s *remoteBranchTestStore) Check(context.Context, model.CleanupSnapshot) error { return nil }
func (s *remoteBranchTestStore) MarkAuditReflected(_ context.Context, snap model.CleanupSnapshot, _ string) (model.CleanupSnapshot, error) {
	return snap, nil
}
func (s *remoteBranchTestStore) Release(_ context.Context, snap model.CleanupSnapshot, _ string) (model.CleanupSnapshot, error) {
	snap.Record.CleanupAttempt = nil
	return snap, nil
}

type remoteBranchTestLifetime struct{}

func (l remoteBranchTestLifetime) Context(ctx context.Context) context.Context    { return ctx }
func (l remoteBranchTestLifetime) Close() error                                   { return nil }
func (l remoteBranchTestLifetime) Drain(context.Context) (CleanupLifetime, error) { return l, nil }

type remoteBranchTestEnvironment struct {
	t     *testing.T
	ctx   context.Context
	calls *[]string
	oid   string
}

func (e *remoteBranchTestEnvironment) call(ctx context.Context, repo, name string) {
	e.t.Helper()
	if ctx != e.ctx || repo != "/repo" {
		e.t.Fatalf("lost caller context or repository: %v %q", ctx, repo)
	}
	*e.calls = append(*e.calls, name)
}
func (e *remoteBranchTestEnvironment) OriginURL(ctx context.Context, repo string) (string, error) {
	e.call(ctx, repo, "origin")
	return "git@github.com:acme/repo.git", nil
}
func (e *remoteBranchTestEnvironment) RemoteRef(ctx context.Context, repo, branch string) (string, error) {
	e.call(ctx, repo, "ref")
	if branch != "123-work" {
		e.t.Fatalf("ref target=%q", branch)
	}
	return e.oid, nil
}
func (e *remoteBranchTestEnvironment) TipReachedBase(ctx context.Context, repo, oid, base string) bool {
	e.call(ctx, repo, "ancestry")
	return false
}
func (e *remoteBranchTestEnvironment) Delete(ctx context.Context, repo, branch, oid string) error {
	e.call(ctx, repo, "delete")
	if branch != "123-work" || oid != e.oid {
		e.t.Fatalf("deletion lost observed target: %q %q", branch, oid)
	}
	return nil
}

func TestRemoteBranchCleanerSnapshotsCompletionBeforeDeletionAndReportsAuditFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := []string{}
	env := &remoteBranchTestEnvironment{t: t, ctx: ctx, calls: &calls, oid: "abc"}
	record := model.IssueOpsRecord{ID: "io-test", Repo: "/repo", Branch: "123-work", Phase: model.IssueOpsPhaseDone, RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/1"}}
	s := RemoteBranchCleaner{
		Records: &remoteBranchTestStore{record},
		Acquire: func(context.Context, string) (CleanupLifetime, error) { return remoteBranchTestLifetime{}, nil },
		NewAttempt: func(operation model.CleanupOperation) (model.IssueOpsCleanupAttempt, error) {
			return model.IssueOpsCleanupAttempt{Operation: operation, Token: "token", StartedAt: "now"}, nil
		},
		Preview: RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: func(artifact model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
			calls = append(calls, "merge")
			if artifact.URL != record.RemoteArtifact.URL {
				t.Fatal("different artifact verified")
			}
			return model.CleanupRemoteBranchArtifactHead{HeadRefName: record.Branch, HeadRefOID: env.oid}, nil
		}},
		Completion: func(got model.IssueOpsRecord) model.RemoteCompletionSection {
			calls = append(calls, "completion")
			if !reflect.DeepEqual(got, record) {
				t.Fatal("completion record drifted")
			}
			return model.RemoteCompletionSection{CleanupAudit: "snapshot marker"}
		},
		ReflectAudit: func(gotCtx context.Context, got model.IssueOpsRecord, completion model.RemoteCompletionSection, audit string) error {
			calls = append(calls, "audit")
			if gotCtx != ctx || !reflect.DeepEqual(got, record) || completion.CleanupAudit != "snapshot marker" {
				t.Fatal("lost completion snapshot or context")
			}
			if audit != "원격 브랜치 삭제: branch=123-work oid=abc at=2026-09-29T00:00:00Z" {
				t.Fatalf("audit=%q", audit)
			}
			return errors.New("provider unavailable")
		},
		Now: func() time.Time { return time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) },
	}
	preview, err := s.Run(ctx, model.CleanupRemoteBranchRequest{ID: record.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, []string{"merge", "origin", "ref"}) {
		t.Fatalf("preview effects=%v", calls)
	}
	calls = nil
	got, err := s.Run(ctx, model.CleanupRemoteBranchRequest{ID: record.ID, Apply: true, Confirm: true, Fingerprint: preview.Fingerprint})
	if err != nil || !got.OK || !got.Deleted || got.AuditReflected || got.AuditError != "provider unavailable" {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	if !reflect.DeepEqual(calls, []string{"merge", "origin", "ref", "completion", "delete", "audit"}) {
		t.Fatalf("apply effects=%v", calls)
	}
}
