package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

// intentArtifactRecord는 plan을 스테이징한 뒤 direct execution과 intent를 붙인
// record를 돌려준다. materializeStagedArtifacts가 intent를 record에서 파생하는
// 경로를 그대로 지난다.
func intentArtifactRecord(t *testing.T, intent *issueopscontract.IssueOpsIntentContract) (string, issueopscontract.IssueOpsRecord, string) {
	t.Helper()
	stateRoot, record := executionPrepareRecord(t)
	if _, err := stageIssueOpsArtifactForTest(stateRoot, record.ID, "plan", []byte("# Owner plan\n")); err != nil {
		t.Fatal(err)
	}
	worktree := t.TempDir()
	record.WorktreePath = worktree
	record.Intent = intent
	record.Execution = &issueopscontract.Execution{
		Mode:      issueopscontract.ExecutionModeDirect,
		Workspace: issueopscontract.Workspace{Root: worktree, ArtifactDir: issueArtifactDirFor(record)},
		Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusActive},
	}
	return stateRoot, record, worktree
}

func sampleIntentContract() *issueopscontract.IssueOpsIntentContract {
	return &issueopscontract.IssueOpsIntentContract{
		RawRequest:        "INTENT.md를 어느 시점에 어느 위치에 만들지 조사해줘",
		InterpretedIntent: "record.intent를 봉인 artifact로 materialize한다",
		SuccessCriteria:   []string{"prepare가 intent.md를 0600으로 쓴다"},
		NonGoals:          []string{"추적 사본 커밋"},
		IntentClass:       "standard",
		RecordedAt:        "2026-09-09T09:00:00Z",
	}
}

func TestMaterializeStagedArtifactsSealsDerivedIntent(t *testing.T) {
	stateRoot, record, worktree := intentArtifactRecord(t, sampleIntentContract())
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(worktree, filepath.FromSlash(issueArtifactDirFor(record)), "intent.md")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		t.Fatalf("intent artifact must be a 0600 regular file: info=%v err=%v", info, err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if manifest["intent"] != digestExecutionOwnerBytes(content) {
		t.Fatalf("manifest intent digest %q does not match file digest %q", manifest["intent"], digestExecutionOwnerBytes(content))
	}
	if !strings.Contains(string(content), "INTENT.md를 어느 시점에 어느 위치에 만들지 조사해줘") || !strings.Contains(string(content), "- issue: https://github.com/acme/repo/issues/16") {
		t.Fatalf("intent artifact must carry the raw request and issue URL:\n%s", content)
	}
	if _, ok := manifest["plan"]; !ok {
		t.Fatalf("plan must still be materialized: %v", manifest)
	}
}

func TestMaterializeStagedArtifactsWithoutIntentKeepsManifest(t *testing.T) {
	stateRoot, record, worktree := intentArtifactRecord(t, nil)
	manifest, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) != 1 || manifest["plan"] == "" {
		t.Fatalf("manifest without intent must carry only plan: %v", manifest)
	}
	if _, err := os.Lstat(filepath.Join(worktree, filepath.FromSlash(issueArtifactDirFor(record)), "intent.md")); !os.IsNotExist(err) {
		t.Fatalf("intent.md must not exist without record.intent: %v", err)
	}
}

func TestMaterializeStagedArtifactsRejectsSecretLikeIntentWithRecordHint(t *testing.T) {
	intent := sampleIntentContract()
	intent.Constraints = []string{"claim token: abc123 를 로그에 남기지 않는다"}
	stateRoot, record, _ := intentArtifactRecord(t, intent)
	_, err := materializeStagedArtifacts(stateRoot, record)
	if err == nil || !strings.Contains(err.Error(), "issueops intent record") || strings.Contains(err.Error(), "staging") {
		t.Fatalf("secret-like intent must fail with an intent record hint and no staging hint: %v", err)
	}
}

func TestMaterializeStagedArtifactsIntentIsImmutableAcrossRerecords(t *testing.T) {
	stateRoot, record, _ := intentArtifactRecord(t, sampleIntentContract())
	first, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil {
		t.Fatal(err)
	}
	// 같은 내용에 기록 시각만 바뀐 재기록: 같은 바이트이므로 통과한다.
	record.Intent.RecordedAt = "2026-09-10T00:00:00Z"
	second, err := materializeStagedArtifacts(stateRoot, record)
	if err != nil || second["intent"] != first["intent"] {
		t.Fatalf("re-materialize with a new recorded_at must reuse the same bytes: err=%v first=%q second=%q", err, first["intent"], second["intent"])
	}
	// 내용이 달라진 재기록: 불변 writer가 거부한다.
	record.Intent.RawRequest = "다른 요청"
	if _, err := materializeStagedArtifacts(stateRoot, record); err == nil || !strings.Contains(err.Error(), "immutable owner artifact already exists with different identity") {
		t.Fatalf("changed intent must be rejected by the immutable writer: %v", err)
	}
}
