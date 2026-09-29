package issueops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/intentdesign"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsintent"
	"issueops/internal/domain/secretdetection"
)

// artifactStageBucket은 prepare 이전에 코디네이터가 스테이징한 artifact를
// 담는다. issueops bucket과 분리되어 있어 훅의 레코드 스캔 비용에 영향이
// 없다(설계 v5 WS2).
const artifactStageBucket = "artifact_stage_v1"

// IssueOpsArtifactDir은 execution.workspace.artifact_dir이 비어 있을 때의
// legacy 봉인 디렉터리다. `.gitignore` 대상이며 보존은 completion 섹션이
// 담당한다. 새 prepare는 application의 OwnerArtifactDir로 이슈 폴더 아래를 고른다(#482).
const IssueOpsArtifactDir = ".issueops/artifact"

// sealedArtifactDir은 레코드가 봉인 아티팩트를 두는 워크트리 상대 디렉터리다.
// 레코드 필드만 본다 — 파일시스템 상태나 PlanPath 파싱으로 추론하지 않는다.
func sealedArtifactDir(record issueops.IssueOpsRecord) string {
	if record.Execution != nil {
		if dir := strings.TrimSpace(record.Execution.Workspace.ArtifactDir); dir != "" {
			return dir
		}
	}
	return IssueOpsArtifactDir
}

// sealedArtifactPath는 워크트리 root 아래 봉인 아티팩트 name.md의 절대 경로다.
func sealedArtifactPath(record issueops.IssueOpsRecord, root, name string) string {
	return filepath.Join(root, filepath.FromSlash(sealedArtifactDir(record)), name+".md")
}

func newPlanResumeArtifactRequiredError(record issueops.IssueOpsRecord) error {
	typed := &domain.OwnerPlanRequiredError{}
	if record.Execution != nil && record.Execution.Lease.Generation > 0 {
		typed.NextCommand = executionReplacementPreviewCommand(record.ID, record.Execution.Lease.Generation)
	}
	return typed
}

func readLinkedPlanIdentity(record issueops.IssueOpsRecord) (issueops.OwnerPlanIdentity, error) {
	worktree := strings.TrimSpace(record.WorktreePath)
	planPath := strings.TrimSpace(record.PlanPath)
	if worktree == "" || planPath == "" || strings.Contains(worktree, "\x00") || strings.Contains(planPath, "\x00") {
		return issueops.OwnerPlanIdentity{}, fmt.Errorf("durable plan path is unavailable")
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(worktree, planPath)
	}
	worktree, err := filepath.Abs(worktree)
	if err != nil {
		return issueops.OwnerPlanIdentity{}, err
	}
	planPath, err = filepath.Abs(planPath)
	if err != nil {
		return issueops.OwnerPlanIdentity{}, err
	}
	worktree, planPath = filepath.Clean(worktree), filepath.Clean(planPath)
	worktreeInfo, err := os.Stat(worktree)
	if err != nil || !worktreeInfo.IsDir() || !issueOpsPlanPathInsideWorktree(worktree, planPath) {
		return issueops.OwnerPlanIdentity{}, fmt.Errorf("durable plan path is outside the canonical worktree")
	}
	info, err := os.Lstat(planPath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return issueops.OwnerPlanIdentity{}, fmt.Errorf("durable plan path is not a regular file")
	}
	content, err := os.ReadFile(planPath)
	if err != nil || strings.TrimSpace(string(content)) == "" {
		return issueops.OwnerPlanIdentity{}, fmt.Errorf("durable plan is empty or unreadable")
	}
	return issueops.OwnerPlanIdentity{Path: planPath, Digest: digestExecutionOwnerBytes(content)}, nil
}

func readStagedArtifacts(stateRoot, id string) (map[string]string, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return nil, err
	}
	data, found, err := db.Get(artifactStageBucket, id)
	if err != nil {
		return nil, err
	}
	staged := map[string]string{}
	if !found || len(data) == 0 {
		return staged, nil
	}
	if err := json.Unmarshal(data, &staged); err != nil {
		return nil, fmt.Errorf("parse staged artifacts: %w", err)
	}
	return staged, nil
}

// materializeStagedArtifacts는 스테이징된 artifact를 워크트리의
// IssueOpsArtifactDir로 0600 파일로 옮기고 name→sha256 manifest를 돌려준다.
// writeExecutionOwnerArtifact의 immutable 계약을 재사용하므로 재실행은
// 동일 내용일 때만 통과한다. generic helper는 스테이징이 없으면 빈 manifest를
// 반환하지만 Orca owner 경로는 application의 MaterializePlan에서 plan을
// 필수로 검증한다. replacement 재봉인도 그 경로로 같은 내용을 다시 검증한다.
// 기존 파일을 바꾸는 재-materialize는 immutable writer가 거부한다.
func materializeStagedArtifacts(stateRoot string, record issueops.IssueOpsRecord) (map[string]string, error) {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return nil, fmt.Errorf("cannot materialize artifacts without a canonical worktree")
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err != nil {
		return nil, err
	}
	manifest := map[string]string{}
	root := record.Execution.Workspace.Root
	for name, content := range staged {
		path := sealedArtifactPath(record, root, name)
		if err := writeExecutionOwnerArtifact(root, path, []byte(content)); err != nil {
			return nil, fmt.Errorf("materialize artifact %s: %w", name, err)
		}
		manifest[name] = digestExecutionOwnerBytes([]byte(content))
	}
	// intent는 staging 대상이 아니라 record.intent에서 파생하는 문서다. 문서에
	// 기록 시각을 넣지 않으므로 같은 내용의 재기록은 같은 바이트가 되고, 내용이
	// 달라진 재봉인은 plan과 똑같이 불변 writer가 거부한다.
	if record.Intent != nil {
		content := []byte(issueopsintent.Render(intentdesign.IntentDocument(record)))
		if secretdetection.Contains(string(content)) {
			// 오류가 staging 표면을 가리키면 사용자가 엉뚱한 곳을 고치므로 record 쪽 명령을 안내한다.
			return nil, fmt.Errorf("materialize artifact intent: record.intent contains secret-like values; rerun `issueops intent record` with the value redacted, then rerun execution prepare")
		}
		if err := writeExecutionOwnerArtifact(root, sealedArtifactPath(record, root, "intent"), content); err != nil {
			return nil, fmt.Errorf("materialize artifact intent: %w", err)
		}
		manifest["intent"] = digestExecutionOwnerBytes(content)
	}
	return manifest, nil
}

// rejectSecretLikeContent는 issueops_decision.go의 거부형 secret 검사 계약을
// artifact 본문에 적용한다.
