package issueops

import (
	issueopspublication "issueops/internal/contract/issueopspublication"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	abandondomain "issueops/internal/domain/issueops"
)

const abandonIssueBodySHA = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

type fakeAbandonGit struct{ branchOID string }

func (g *fakeAbandonGit) run(_ string, args ...string) (int, string) {
	if len(args) > 0 && args[0] == "rev-parse" {
		if g.branchOID == "" {
			return 1, ""
		}
		return 0, g.branchOID
	}
	return 0, ""
}

func abandonTestRecord(t *testing.T) (string, issueops.IssueOpsRecord) {
	t.Helper()
	stateRoot := filepath.Join(t.TempDir(), "issueops")
	repo := t.TempDir()
	record, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: repo, Branch: "106-abandon"})
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, record
}

func abandonRequest(id string, apply bool, fingerprint string) issueops.CleanupAbandonRequest {
	return issueops.CleanupAbandonRequest{
		ID: id, Reason: "폐기된 비-done 사이클 정리",
		Apply: apply, Confirm: apply, Fingerprint: fingerprint,
	}
}

func abandonDeps(git *fakeAbandonGit) CleanupAbandonDeps {
	return CleanupAbandonDeps{Processes: quietCleanupProcesses(), Git: git.run}
}

func writeAbandonIntentRow(t *testing.T, stateRoot, operationID, lifecycleID string) {
	t.Helper()
	data, err := json.Marshal(map[string]string{"operation_id": operationID, "lifecycle_id": lifecycleID})
	if err != nil {
		t.Fatal(err)
	}
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(externalIntentBucket, operationID, data); err != nil {
		t.Fatal(err)
	}
}

func abandonIntentRowExists(t *testing.T, stateRoot, operationID string) bool {
	t.Helper()
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		t.Fatal(err)
	}
	_, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}

// abandonExecution은 지정한 lease로 released-가능한 실행 블록을 만든다.
// root는 부모만 존재하고 자기 자신은 없는 경로다(worktree_absent 충족).
func abandonExecution(repo, root string, lease issueops.WriteLease) *issueops.Execution {
	return &issueops.Execution{
		Workspace: issueops.Workspace{
			SourceRoot: repo, Root: root, Branch: "106-abandon",
			BaseHead: "deadbeef", LinkedAt: "2026-07-24T00:00:00Z",
		},
		Lease: lease,
	}
}

// abandonIntentRecord는 실측 형태를 재현한다: released lease +
// external_operation_ambiguous failure + 디스크에 없는 workspace root. pendingKind가
// 비어 있지 않으면 같은 operation의 pending intent도 건다.
func abandonIntentRecord(t *testing.T, pendingKind string, writeRow bool) (string, issueops.IssueOpsRecord, string, string) {
	t.Helper()
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "absent-worktree")
	operationID := "op-abandon-intent"
	if writeRow {
		writeAbandonIntentRow(t, stateRoot, operationID, record.ID)
	}
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		if pendingKind != "" {
			rec.Execution.Pending = &issueops.ExternalIntent{
				OperationID: operationID, Kind: pendingKind, Marker: "marker", StartedAt: "2026-07-24T00:00:00Z",
			}
		}
		rec.Execution.Failure = &issueops.ExecutionFailure{
			OperationID: operationID, Code: "external_operation_ambiguous", At: "2026-07-24T00:00:00Z",
		}
	})
	updated, err := ReadIssueOps(stateRoot, record.ID)
	if err != nil {
		t.Fatal(err)
	}
	return stateRoot, updated, operationID, root
}

// AC-02 성공 경로: preview가 삭제 대상 레코드 전문과 fingerprint를 내고,
// apply가 레코드를 삭제하며, 동일 (repo, branch) start가 새 사이클을 연다.
func TestCleanupAbandonPreviewThenApplyDeletesRecord(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	deps := abandonDeps(&fakeAbandonGit{})

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !preview.Preview || preview.Fingerprint == "" || preview.RecordDeleted {
		t.Fatalf("preview must issue a fingerprint without mutating: %+v", preview)
	}
	// design-review F7 완화책: 삭제 전 캡처가 가능해야 한다.
	if preview.Record == nil || preview.Record.ID != record.ID || preview.Record.Repo != record.Repo {
		t.Fatalf("preview must carry the full record about to be deleted: %+v", preview.Record)
	}
	if preview.Reason != "폐기된 비-done 사이클 정리" {
		t.Fatalf("preview must echo the reason: %q", preview.Reason)
	}

	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.RecordDeleted || applied.AbandonedAt == "" || applied.Record == nil {
		t.Fatalf("apply must delete the record and echo the snapshot and time: %+v", applied)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record must be gone after abandon: %v", err)
	}
	fresh, err := startIssueOpsFixture(stateRoot, issueops.IssueOpsStartRequest{Repo: record.Repo, Branch: "106-abandon"})
	if err != nil {
		t.Fatal(err)
	}
	if fresh.Phase != issueops.IssueOpsPhaseProblem || fresh.Execution != nil {
		t.Fatalf("abandon must unlock same-branch rework with a fresh cycle: %+v", fresh)
	}
}

// AC-02: LeaseStatus 4값 전수 테이블. released만 통과하고 revoking을 포함한
// 나머지는 전부 lease_terminal로 거부된다(design-review F5).
func TestCleanupAbandonLeaseAllowlistCoversEveryStatus(t *testing.T) {
	holder := &issueops.NativeActor{
		Host: "codex", SessionID: "s-1",
		SessionProcess: &issueops.NativeProcessReceipt{PID: 4321, StartedAt: "2026-07-24T00:00:00Z", Executable: "/usr/bin/codex"},
	}
	cases := []struct {
		name  string
		lease issueops.WriteLease
		ready bool
	}{
		{"released", issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased}, true},
		// claimable은 #140에서 통과로 바뀌었다. validateWriteLease가 홀더 부재를
		// 강제하므로 released와 같은 성질이고, 거부하면 운영자가 claim→release로
		// lease를 한 바퀴 돌리는 우회를 하게 된다(#139에서 실측). 자원 잔여는
		// 게이트 ⑤·⑥·⑦·⑨가 각각 막는다.
		{"claimable", issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusClaimable, ClaimTokenSHA256: abandonIssueBodySHA}, true},
		{"active", issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusActive, Holder: holder, ClaimedAt: "2026-07-24T00:00:00Z"}, false},
		{"revoking", issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusRevoking, Holder: holder}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := abandonTestRecord(t)
			root := filepath.Join(t.TempDir(), "absent-worktree")
			mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
				rec.Execution = abandonExecution(rec.Repo, root, tc.lease)
			})
			deps := abandonDeps(&fakeAbandonGit{})
			result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
			if tc.ready {
				if err != nil || result.Fingerprint == "" {
					t.Fatalf("released lease must be ready: %v %+v", err, result)
				}
				return
			}
			if err == nil || !containsString(result.Missing, "lease_terminal") {
				t.Fatalf("%s lease must block abandon: %v %v", tc.name, err, result.Missing)
			}
		})
	}
}

// AC-02: 나머지 레코드 게이트(phase/artifact/children/reason).
func TestCleanupAbandonRecordGatesRejectUnsafeRecords(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*issueops.IssueOpsRecord)
		request func(id string) issueops.CleanupAbandonRequest
		missing string
	}{
		{
			// done + 머지 증적은 여전히 reflect→finish의 몫이다. 미병합 관측이
			// 없으면 phase가 done이든 아니든 게이트는 닫힌 채로 남는다(#342).
			name: "done phase with an artifact belongs to finish",
			mutate: func(rec *issueops.IssueOpsRecord) {
				rec.Phase = issueops.IssueOpsPhaseDone
				rec.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
					Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/9",
				}
			},
			missing: "remote_artifact_unmerged",
		},
		{
			name: "remote artifact belongs to reflect then finish",
			mutate: func(rec *issueops.IssueOpsRecord) {
				rec.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{
					Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/9",
				}
			},
			missing: "remote_artifact_unmerged",
		},
		{
			name: "child cycles would be orphaned",
			mutate: func(rec *issueops.IssueOpsRecord) {
				rec.ChildCycles = append(rec.ChildCycles, issueops.IssueOpsChildCycleRef{CycleID: "io-child000000", Branch: "child"})
			},
			missing: "no_children",
		},
		{
			name: "unclosed child issue link",
			mutate: func(rec *issueops.IssueOpsRecord) {
				rec.IssueLinks = append(rec.IssueLinks, issueops.IssueOpsIssueLink{Type: "child", URL: "https://github.com/acme/repo/issues/91"})
			},
			missing: "no_children",
		},
		{
			name: "blank reason",
			request: func(id string) issueops.CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = "   "
				return r
			},
			missing: "reason_required",
		},
		{
			name: "control character in reason",
			request: func(id string) issueops.CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = "abandon\nnow"
				return r
			},
			missing: "reason_required",
		},
		{
			name: "active shell character in reason",
			request: func(id string) issueops.CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = "abandon $(rm -rf /)"
				return r
			},
			missing: "reason_required",
		},
		{
			name: "reason over the byte limit",
			request: func(id string) issueops.CleanupAbandonRequest {
				r := abandonRequest(id, false, "")
				r.Reason = strings.Repeat("a", abandondomain.CleanupAbandonReasonLimit+1)
				return r
			},
			missing: "reason_required",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stateRoot, record := abandonTestRecord(t)
			if tc.mutate != nil {
				mutateFinishRecord(t, stateRoot, record.ID, tc.mutate)
			}
			req := abandonRequest(record.ID, false, "")
			if tc.request != nil {
				req = tc.request(record.ID)
			}
			deps := abandonDeps(&fakeAbandonGit{})
			result, err := CleanupAbandon(context.Background(), stateRoot, req, deps)
			if err == nil || !containsString(result.Missing, tc.missing) {
				t.Fatalf("expected missing %q: err=%v missing=%v", tc.missing, err, result.Missing)
			}
		})
	}
}

func TestCleanupAbandonIgnoresSelfChildIssueLink(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.IssueURL = "https://github.com/acme/repo/issues/91"
		rec.IssueLinks = append(rec.IssueLinks, issueops.IssueOpsIssueLink{
			Type: "child",
			URL:  rec.IssueURL,
		})
	})

	result, err := CleanupAbandon(
		context.Background(),
		stateRoot,
		abandonRequest(record.ID, false, "superseded self-linked retry"),
		abandonDeps(&fakeAbandonGit{}),
	)
	if err != nil {
		t.Fatalf("self child link must not block abandon: %v missing=%v", err, result.Missing)
	}
	if result.Fingerprint == "" {
		t.Fatal("ready abandon preview must issue a fingerprint")
	}
}

// AC-02: 로컬 잔여물(워크트리 디렉토리, 브랜치 ref, 경로 불일치)은 abandon을
// 막는다 — abandon은 아무것도 지우지 않는 경로이기 때문이다.
func TestCleanupAbandonRejectsLocalResidue(t *testing.T) {
	t.Run("worktree present", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "live-worktree")
		if err := os.MkdirAll(root, 0o755); err != nil {
			t.Fatal(err)
		}
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		})
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}))
		// registry가 인정하지 않는 worktree는 계속 막는다. 비대칭 자체는
		// #433 이후 거부 사유가 아니므로 worktree_canonical만 남는다.
		if err == nil || !containsString(result.Missing, "worktree_canonical") {
			t.Fatalf("unverified worktree-only residue must block: %v %v", err, result.Missing)
		}
		if containsString(result.Missing, "local_residue_pair") {
			t.Fatalf("비대칭 자체는 더 이상 거부 사유가 아니다: %v", result.Missing)
		}
	})
	// branch만 남은 잔여물은 #433 이후 정리 가능하다. 그 계약은
	// TestCleanupAbandonClearsAsymmetricResidue가 고정한다. 여기서는 소유 근거
	// 없는 잔여물이 여전히 막힘을 고정한다 — record가 워크트리를 link하지도
	// 않았고 execution도 없으면 무엇을 지우는 것인지 설명할 수 없다. record가
	// link한 잔여물의 정리는 TestCleanupAbandonAcceptsRecordLinkedResidueWithoutExecution이
	// 고정한다.
	t.Run("branch ref present without execution", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{branchOID: "abc123"}))
		if err == nil || !containsString(result.Missing, "local_residue_execution") {
			t.Fatalf("execution 없는 잔여물은 계속 막아야 한다: %v %v", err, result.Missing)
		}
	})
	t.Run("worktree identity conflict", func(t *testing.T) {
		stateRoot, record := abandonTestRecord(t)
		root := filepath.Join(t.TempDir(), "absent-worktree")
		mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
			rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
			rec.WorktreePath = filepath.Join(t.TempDir(), "other-worktree")
		})
		result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}))
		if err == nil || !containsString(result.Missing, "worktree_identity_conflict") {
			t.Fatalf("conflicting worktree identity must block: %v %v", err, result.Missing)
		}
	})
}

// AC-02: 미해결 pending intent는 abandon을 막고 reconcile로 안내한다.
func TestCleanupAbandonPendingIntentGate(t *testing.T) {
	stateRoot, record, _, _ := abandonIntentRecord(t, issueopspublication.RemoteIntentKind, true)
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), abandonDeps(&fakeAbandonGit{}))
	if err == nil || !containsString(result.Missing, "pending_intent_safe") {
		t.Fatalf("remote pending kind must block: %v %v", err, result.Missing)
	}
	if !strings.Contains(result.PendingIntentError, "execution reconcile") {
		t.Fatalf("rejection must point at execution reconcile: %q", result.PendingIntentError)
	}
}

// AC-01/AC-02 성공 경로: 실측 형태(ambiguous failure)가 preview·apply를 통과하고,
// 레코드와 intent 행을 같은 배치로 지운다.
func TestCleanupAbandonDeletesRecordAndIntentRowAtomically(t *testing.T) {
	stateRoot, record, operationID, _ := abandonIntentRecord(t, "", true)
	deps := abandonDeps(&fakeAbandonGit{})

	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	wantPlanTail := []string{"record:" + record.ID, "intent:" + operationID}
	if len(preview.RemovalPlan) < 2 || strings.Join(preview.RemovalPlan[len(preview.RemovalPlan)-2:], "\n") != strings.Join(wantPlanTail, "\n") {
		t.Fatalf("preview must order record before its owned intent rows: %+v", preview.RemovalPlan)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatal(err)
	}
	if !applied.RecordDeleted || len(applied.IntentRowsDeleted) != 1 || applied.IntentRowsDeleted[0] != operationID {
		t.Fatalf("apply must delete the record and its intent row: %+v", applied)
	}
	if abandonIntentRowExists(t, stateRoot, operationID) {
		t.Fatal("external intent row must not survive the record")
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("record must be gone: %v", err)
	}
}

// 멱등: Failure만 참조하는 행이 이미 없으면 성공이다.
func TestCleanupAbandonTreatsAbsentIntentRowAsSuccess(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	root := filepath.Join(t.TempDir(), "absent-worktree")
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution = abandonExecution(rec.Repo, root, issueops.WriteLease{Generation: 1, Status: issueops.LeaseStatusReleased})
		rec.Execution.Failure = &issueops.ExecutionFailure{
			OperationID: "op-already-collected", Code: "external_operation_ambiguous", At: "2026-07-24T00:00:00Z",
		}
	})
	deps := abandonDeps(&fakeAbandonGit{})
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err != nil {
		t.Fatalf("absent intent row must be idempotent success: %v", err)
	}
	if !applied.RecordDeleted || len(applied.IntentRowsDeleted) != 0 {
		t.Fatalf("no row should be reported deleted: %+v", applied)
	}
}

// 소유자 가드: 다른 lifecycle이 소유한 행은 하드 에러이고 레코드는 보존된다
// (execution_state.go:150-159 규율 준용).
func TestCleanupAbandonRefusesToDeleteAnotherLifecyclesIntentRow(t *testing.T) {
	stateRoot, record, operationID, _ := abandonIntentRecord(t, "", true)
	foreign := "op-foreign-row"
	writeAbandonIntentRow(t, stateRoot, foreign, "io-someoneelse")
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) {
		rec.Execution.Failure.OperationID = foreign
	})
	deps := abandonDeps(&fakeAbandonGit{})
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	result, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || !strings.Contains(err.Error(), "another lifecycle") || result.FailedStep != "record_delete" {
		t.Fatalf("foreign intent row must hard-fail: %v %+v", err, result)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("record must survive the refusal: %v", err)
	}
	if !abandonIntentRowExists(t, stateRoot, operationID) || !abandonIntentRowExists(t, stateRoot, foreign) {
		t.Fatal("no row may be deleted when the batch is refused")
	}
}

// TOCTOU: preview 이후 fingerprint 입력이 바뀌면 apply는 거부된다. apply
// without --confirm도 함께 고정한다.
func TestCleanupAbandonApplyRejectsStaleFingerprintAndMissingConfirm(t *testing.T) {
	stateRoot, record := abandonTestRecord(t)
	deps := abandonDeps(&fakeAbandonGit{})
	preview, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, false, ""), deps)
	if err != nil {
		t.Fatal(err)
	}
	noConfirm := abandonRequest(record.ID, true, preview.Fingerprint)
	noConfirm.Confirm = false
	if _, err := CleanupAbandon(context.Background(), stateRoot, noConfirm, deps); err == nil || !strings.Contains(err.Error(), "--confirm") {
		t.Fatalf("apply without confirm must be rejected: %v", err)
	}
	// phase 이동은 게이트를 통과하지만 fingerprint 입력을 바꾼다.
	mutateFinishRecord(t, stateRoot, record.ID, func(rec *issueops.IssueOpsRecord) { rec.Phase = issueops.IssueOpsPhasePlan })
	if _, err := CleanupAbandon(context.Background(), stateRoot, abandonRequest(record.ID, true, preview.Fingerprint), deps); err == nil ||
		!strings.Contains(err.Error(), "stale cleanup fingerprint") {
		t.Fatalf("stale fingerprint must be rejected: %v", err)
	}
	if _, err := ReadIssueOps(stateRoot, record.ID); err != nil {
		t.Fatalf("record must survive a stale apply: %v", err)
	}
}

func TestCleanupAbandonRejectsUnknownBranchInventory(t *testing.T) {
	for _, code := range []int{2, 128, -1} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			root, record := abandonTestRecord(t)
			deps := abandonDeps(&fakeAbandonGit{})
			deps.Git = func(_ string, args ...string) (int, string) {
				if args[0] == "rev-parse" {
					return code, "unavailable"
				}
				return 0, ""
			}
			preview, err := CleanupAbandon(context.Background(), root, abandonRequest(record.ID, false, ""), deps)
			if err == nil || preview.OK || preview.Fingerprint != "" {
				t.Errorf("unknown branch authorized preview: %+v %v", preview, err)
			}
			got, err := CleanupAbandon(context.Background(), root, abandonRequest(record.ID, true, preview.Fingerprint), deps)
			if err == nil || got.OK || got.RecordDeleted {
				t.Errorf("unknown branch authorized apply: %+v %v", got, err)
			}
			if _, err := ReadIssueOps(root, record.ID); err != nil {
				t.Fatalf("unknown branch lost record: %v", err)
			}
		})
	}
}

func TestCleanupAbandonRejectsNonDirectoryWorkspace(t *testing.T) {
	root, record := abandonTestRecord(t)
	path := filepath.Join(t.TempDir(), "workspace")
	if err := os.WriteFile(path, []byte("preserve"), 0600); err != nil {
		t.Fatal(err)
	}
	mutateFinishRecord(t, root, record.ID, func(r *issueops.IssueOpsRecord) { r.WorktreePath = path })
	deps := abandonDeps(&fakeAbandonGit{})
	preview, err := CleanupAbandon(context.Background(), root, abandonRequest(record.ID, false, ""), deps)
	if err == nil || preview.OK || preview.Fingerprint != "" {
		t.Errorf("file accepted as absent workspace: %+v %v", preview, err)
	}
	got, err := CleanupAbandon(context.Background(), root, abandonRequest(record.ID, true, preview.Fingerprint), deps)
	if err == nil || got.OK || got.RecordDeleted {
		t.Errorf("file allowed record deletion: %+v %v", got, err)
	}
	if _, err := ReadIssueOps(root, record.ID); err != nil {
		t.Fatalf("file workspace lost record: %v", err)
	}
}
