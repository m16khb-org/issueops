// Package gatesgate는 IssueOps의 PR readiness에 태스크 게이트 ledger 게이트를
// 합성한다.
//
// loopgate가 loop run 상태를 readiness에 더하는 것과 같은 조립 구조다.
// 게이트 파일(worktree의 .issueops/gates/*.md 또는 호환 경로)이 존재하면 미충족 게이트가
// PR 진입을 막고, 파일이 없으면 게이트가 적용되지 않는다 — unlazy와 같은
// opt-in: 게이트를 만드는 순간 완료가 구조적으로 강제된다.
package gatesgate

import (
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/loopgate"
	cycleapp "issueops/internal/application/issueopscycle"
	gatescontract "issueops/internal/contract/gates"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	issueopsremote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/stringlist"
	cycleport "issueops/internal/port/issueopscycle"
)

// gates ledger 조회·평가는 composition root가 설치한다. gatesgate는 gates
// adapter를 직접 import하지 않는다(크로스 케퍼빌리티 adapter edge 금지,
// loopgate의 RepoGateMissing 패턴과 같다).
var (
	// DiscoverGateFiles는 canonical/compatible 게이트 파일 발견 연산이다.
	DiscoverGateFiles func(root string) ([]string, error)
	// CheckGateLedger는 게이트 ledger 평가 연산이다.
	CheckGateLedger func(req gatescontract.CheckRequest) (gatescontract.CheckResult, error)
)

// StrictPRReadinessWithState는 loop 게이트에 게이트 ledger 게이트를 더한
// strict readiness다.
func StrictPRReadinessWithState(stateRoot string, record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
	ready := loopgate.StrictPRReadinessWithState(stateRoot, record)
	ready = withGatesGate(ready, GatesRootFor(record), linkedIssueNumber(record))
	return withDuplicateIssueArtifactGate(ready, GatesRootFor(record), linkedIssueNumber(record))
}

// linkedIssueNumber는 레코드가 가리키는 provider 이슈 번호다. 없으면 빈 문자열.
func linkedIssueNumber(record issueopscontract.IssueOpsRecord) string {
	if n := issueopsremote.IssueNumber(record.IssueURL); n != "" {
		return n
	}
	if record.BranchPrepare != nil {
		return issueopsremote.IssueNumber(record.BranchPrepare.IssueURL)
	}
	return ""
}

// withDuplicateIssueArtifactGate는 현재 사이클의 이슈 원장이 canonical
// `.issueops/issues/<n>/gates.md`와 호환 경로 `.issueops/gates/`
// (`issue-<n>*`, `<n>-*`) 양쪽에 있으면 `duplicate_issue_artifact:<n>`으로
// fail-closed한다(#480). 다른 이슈의 중복은 이 사이클을 막지 않으며, 번호를
// 모르면 판정하지 않는다.
func withDuplicateIssueArtifactGate(ready issueopscontract.IssueOpsReadiness, root, issueNumber string) issueopscontract.IssueOpsReadiness {
	root, issueNumber = strings.TrimSpace(root), strings.TrimSpace(issueNumber)
	if root == "" || issueNumber == "" {
		return ready
	}
	canonical := filepath.Join(root, ".issueops", "issues", issueNumber, "gates.md")
	if info, err := os.Stat(canonical); err != nil || info.IsDir() {
		return ready
	}
	entries, err := os.ReadDir(filepath.Join(root, ".issueops", "gates"))
	if err != nil {
		return ready
	}
	legacyEntries := make([]issueopsdomain.GateLedgerFile, 0, len(entries))
	for _, entry := range entries {
		legacyEntries = append(legacyEntries, issueopsdomain.GateLedgerFile{Name: entry.Name(), Directory: entry.IsDir()})
	}
	if missing := issueopsdomain.DuplicateGateLedgerMissing(issueNumber, true, legacyEntries); len(missing) > 0 {
		ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), missing...))
		ready.Ready = false
	}
	return ready
}

// AdvancePhaseWithActor는 pr 단계 진입 전에 게이트 ledger까지 포함한 strict
// readiness를 강제한다.
func AdvancePhaseWithActor(stateRoot, id, to string, actor issueops.IssueOpsActor) (issueopscontract.IssueOpsRecord, error) {
	if err := guardPRPhase(stateRoot, id, to); err != nil {
		return issueopscontract.IssueOpsRecord{OK: false}, err
	}
	return issueops.AdvanceIssueOpsPhaseWithActor(stateRoot, id, to, actor)
}

// GatesRootFor는 게이트 파일을 찾을 루트다. worktree가 있으면 worktree, 없으면
// 레코드 repo를 쓴다.
func GatesRootFor(record issueopscontract.IssueOpsRecord) string {
	if path := strings.TrimSpace(record.WorktreePath); path != "" {
		return path
	}
	return strings.TrimSpace(record.Repo)
}

// guardPRPhase는 이미 pr 단계인 레코드는 통과시킨다(복구 경로 보존). core
// readiness는 AdvanceIssueOpsPhaseWithActor가 upstream을 span 밖에서 fetch한 뒤
// span 안에서 판정하므로, 여기서는 이 package가 합성하는 loop·게이트 ledger·
// 중복 원장 게이트만 본다. core 판정을 여기서 다시 하면 fetch가 두 번 일어난다.
func guardPRPhase(stateRoot, id, to string) error {
	return cycleapp.GuardPRPhase(stateRoot, id, to, cycleport.PRPhaseGuard{
		Read: issueops.ReadIssueOps,
		Gate: func(record issueopscontract.IssueOpsRecord) issueopscontract.IssueOpsReadiness {
			ready := loopgate.WithLoopGate(issueopscontract.IssueOpsReadiness{Ready: true}, record.Repo)
			ready = withGatesGate(ready, GatesRootFor(record), linkedIssueNumber(record))
			return withDuplicateIssueArtifactGate(ready, GatesRootFor(record), linkedIssueNumber(record))
		},
	})
}

func withGatesGate(ready issueopscontract.IssueOpsReadiness, root, issueNumber string) issueopscontract.IssueOpsReadiness {
	return cycleapp.ApplyGateLedgers(ready, root, issueNumber, cycleport.GateLedgerReadiness{
		Discover: DiscoverGateFiles, Check: CheckGateLedger,
	})
}
