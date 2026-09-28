package issueops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/implementation"
	cycleapp "issueops/internal/application/issueopscycle"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/projectdoc"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordIssueOpsProjectDocsReview는 publication 직전 project-doc 반영 판정을
// 기록한다. verdict가 updated면 적어 낸 문서가 실제 변경 집합 안에 있어야
// 하므로, 문서를 고치지 않고 "갱신했다"고 기록하는 경로가 막힌다.
func RecordIssueOpsProjectDocsReview(stateRoot, id string, req IssueOpsProjectDocsReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, nil)
}

// RecordIssueOpsProjectDocsReviewWithActor는 활성 lease가 있으면 그 holder만
// 기록하게 한다.
func RecordIssueOpsProjectDocsReviewWithActor(stateRoot, id string, req IssueOpsProjectDocsReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsProjectDocsReview(stateRoot, id, req, &actor)
}

func recordIssueOpsProjectDocsReview(stateRoot, id string, req IssueOpsProjectDocsReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return reviewapp.RecordProjectDocsReview(reviewport.ProjectDocsReviewStore{
		EvidenceReviewStore:   reviewEvidenceStore(actor),
		NormalizeDocs:         normalizeProjectDocPaths,
		NormalizeReviewedDocs: normalizeReviewedDocPaths,
	}, stateRoot, id, req)
}

// normalizeProjectDocPaths는 입력 경로를 repo-상대 slash 경로로 정규화하고,
// 각 경로가 현재 변경 집합에 실제로 들어 있는지 확인한다.
func normalizeProjectDocPaths(record issueops.IssueOpsRecord, docs []string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	root := issueOpsStrictGitRoot(record)
	changed := map[string]bool{}
	for _, path := range implementation.ChangedPaths(record) {
		changed[path] = true
	}
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		rel := relativeChangePath(root, doc)
		if rel == "" {
			return nil, fmt.Errorf("project docs review path %q must be inside the worktree", doc)
		}
		if !changed[rel] {
			return nil, fmt.Errorf("project docs review lists %s but it is not in the current change set", rel)
		}
		out = append(out, rel)
	}
	return out, nil
}

// normalizeReviewedDocPaths는 읽었다고 신고한 문서가 실제 project doc이고 지금
// 존재하는지 확인한다. 변경 집합 안에 있을 필요는 없다 — 읽기만 한 문서가
// 대부분이다. 허용 범위는 `.issueops/` 아래와 루트 `AGENTS.md`다.
func normalizeReviewedDocPaths(record issueops.IssueOpsRecord, docs []string) ([]string, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	root := issueOpsStrictGitRoot(record)
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		rel := relativeChangePath(root, doc)
		if rel == "" {
			return nil, fmt.Errorf("project docs review reviewed path %q must be inside the worktree", doc)
		}
		if rel != "AGENTS.md" && !strings.HasPrefix(rel, projectdoc.ProjectDocsDir+"/") {
			return nil, fmt.Errorf("project docs review reviewed path %s is not a project doc (expected %s/... or AGENTS.md)", rel, projectdoc.ProjectDocsDir)
		}
		if root == "" {
			return nil, fmt.Errorf("project docs review cannot verify reviewed path %s without a worktree or repo root", rel)
		}
		info, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
		if statErr != nil || info.IsDir() {
			return nil, fmt.Errorf("project docs review reviewed path %s does not exist as a file", rel)
		}
		out = append(out, rel)
	}
	return out, nil
}

func relativeChangePath(root, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	if filepath.IsAbs(path) {
		if root == "" {
			return ""
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return ""
		}
		path = rel
	}
	path = filepath.ToSlash(filepath.Clean(filepath.FromSlash(path)))
	if path == "." || path == ".." || strings.HasPrefix(path, "../") {
		return ""
	}
	return path
}

// projectDocsReviewMissing은 publication 게이트 판정이다. implementation review와
// 달리 execution mode도, execution lease 유무도 가리지 않는다 — 어떤 경로로
// implement 이후 phase에 왔든 운영 문서에 남길 결정을 만들 수 있기 때문이다.
func projectDocsReviewMissing(record issueops.IssueOpsRecord, currentFingerprint string) string {
	return cycleapp.ProjectDocsReviewMissing(record, currentFingerprint)
}
