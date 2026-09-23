package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
)

// 세 evidence 기록은 PR readiness 게이트가 읽는 owner mutation이다. 활성 lease가
// 있으면 다른 mutation과 같이 holder만 기록할 수 있어야 한다
// (architecture/issueops.md "Every mutating transition requires the active
// generation and matching native actor/cwd"). hook은 이 경계를 대신 막지 않는다.
func TestEvidenceRecordersRequireTheActiveLeaseHolder(t *testing.T) {
	recorders := []struct {
		name   string
		record func(stateRoot, id string, actor *IssueOpsActor) error
	}{
		{"implementation-review", func(stateRoot, id string, actor *IssueOpsActor) error {
			req := IssueOpsImplementationReviewRequest{Verdict: "pass", Findings: []string{"none"}, Evidence: []string{"go test ./..."}}
			if actor == nil {
				_, err := RecordIssueOpsImplementationReview(stateRoot, id, req)
				return err
			}
			_, err := RecordIssueOpsImplementationReviewWithActor(stateRoot, id, req, *actor)
			return err
		}},
		{"schema-evidence", func(stateRoot, id string, actor *IssueOpsActor) error {
			req := IssueOpsSchemaEvidenceRequest{Measurements: []string{"orders rows=1"}, Sources: []string{"psql"}}
			if actor == nil {
				_, err := RecordIssueOpsSchemaEvidence(stateRoot, id, req)
				return err
			}
			_, err := RecordIssueOpsSchemaEvidenceWithActor(stateRoot, id, req, *actor)
			return err
		}},
		{"project-docs-review", func(stateRoot, id string, actor *IssueOpsActor) error {
			req := IssueOpsProjectDocsReviewRequest{Verdict: "no-change", ReviewedDocs: []string{"AGENTS.md"}, Evidence: []string{"read AGENTS.md"}}
			if actor == nil {
				_, err := RecordIssueOpsProjectDocsReview(stateRoot, id, req)
				return err
			}
			_, err := RecordIssueOpsProjectDocsReviewWithActor(stateRoot, id, req, *actor)
			return err
		}},
	}
	for _, recorder := range recorders {
		t.Run(recorder.name, func(t *testing.T) {
			stateRoot := t.TempDir()
			fixture, holder := activeLeaseEvidenceFixture(t, stateRoot)
			if err := os.WriteFile(filepath.Join(fixture.worktree, "AGENTS.md"), []byte("# agents\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			id := fixture.record.ID

			intruder := IssueOpsActor{Host: "claude", SessionID: "intruder", CWD: fixture.worktree}
			if err := recorder.record(stateRoot, id, &intruder); err == nil || !strings.Contains(err.Error(), "requires the current write lease holder") {
				t.Fatalf("a non-holder must not record %s under an active lease: %v", recorder.name, err)
			}
			if err := recorder.record(stateRoot, id, nil); err == nil || !strings.Contains(err.Error(), "requires the current write lease holder") {
				t.Fatalf("a caller without actor must not record %s under an active lease: %v", recorder.name, err)
			}
			wrongCWD := holder
			wrongCWD.CWD = filepath.Dir(fixture.worktree)
			if err := recorder.record(stateRoot, id, &wrongCWD); err == nil || !strings.Contains(err.Error(), "canonical worktree cwd") {
				t.Fatalf("the holder outside the canonical worktree must not record %s: %v", recorder.name, err)
			}
			untouched, err := ReadIssueOps(stateRoot, id)
			if err != nil {
				t.Fatal(err)
			}
			if untouched.ImplementationReview != nil || untouched.SchemaEvidence != nil || untouched.ProjectDocsReview != nil {
				t.Fatalf("refused %s attempts must not write evidence: %+v", recorder.name, untouched)
			}

			if err := recorder.record(stateRoot, id, &holder); err != nil {
				t.Fatalf("the active lease holder must record %s: %v", recorder.name, err)
			}
		})
	}
}

// execution이 아직 없는 레코드(준비 전)는 다른 owner mutation처럼 actor 없이도
// 기록할 수 있다. fence는 execution이 생긴 뒤의 쓰기 권한을 다룬다.
func TestEvidenceRecordersStayOpenBeforeExecutionPreparation(t *testing.T) {
	stateRoot := t.TempDir()
	repo := initIssueOpsRepo(t)
	record, err := StartIssueOps(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "995-pre-execution"})
	if err != nil {
		t.Fatal(err)
	}
	record.Phase = issueops.IssueOpsPhaseImplement
	if _, err := writeIssueOps(stateRoot, record); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordIssueOpsSchemaEvidence(stateRoot, record.ID, IssueOpsSchemaEvidenceRequest{
		Measurements: []string{"orders rows=1"}, Sources: []string{"psql"},
	}); err != nil {
		t.Fatalf("a record without execution must accept schema evidence: %v", err)
	}
}

// activeLeaseEvidenceFixture는 implement 단계에서 다른 세션이 활성 lease를 쥔
// 레코드와, 그 holder로 인정되는 actor를 만든다.
func activeLeaseEvidenceFixture(t *testing.T, stateRoot string) (claimableExecutionFixture, IssueOpsActor) {
	t.Helper()
	fixture := newClaimableExecutionFixture(t, stateRoot, "996-evidence-fence")
	receipt := issueops.NativeProcessReceipt{PID: 4242, StartedAt: "2026-09-23T00:00:00Z", Executable: "/usr/bin/codex"}
	fixture.record.Phase = issueops.IssueOpsPhaseImplement
	fixture.record.Execution.Lease = issueops.WriteLease{
		Generation: 1, Status: issueops.LeaseStatusActive,
		Holder:    &issueops.NativeActor{Host: "codex", SessionID: "owner-session", SessionProcess: &receipt},
		ClaimedAt: "2026-09-23T00:00:00Z",
	}
	written, err := writeIssueOps(stateRoot, fixture.record)
	if err != nil {
		t.Fatal(err)
	}
	fixture.record = written
	if err := os.WriteFile(filepath.Join(fixture.worktree, "changed.go"), []byte("package changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	holder := IssueOpsActor{
		Host: "codex", SessionID: "owner-session", CWD: fixture.worktree,
		NativeProcessAncestry: []issueops.NativeProcessReceipt{receipt},
	}
	return fixture, holder
}
