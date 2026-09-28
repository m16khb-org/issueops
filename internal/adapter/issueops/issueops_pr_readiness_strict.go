package issueops

import (
	"strings"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

// IssueOpsLocalPRReadiness는 네트워크를 건드리지 않는 판정이다. strict에서
// `git fetch`와 그 결과에 기대는 upstream 동기화 판정만 뺐다. 단계 분류처럼
// 자주 부르는 읽기 전용 표면이 원격을 때리지 않게 하려는 분리다.
func IssueOpsLocalPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	ready, _ := ObserveIssueOpsLocalPRReadiness(record)
	return ready
}

// ObserveIssueOpsLocalPRReadiness returns the verified local change
// observation used by readiness so the same request can classify review tier
// without reading Git again.
func ObserveIssueOpsLocalPRReadiness(record issueops.IssueOpsRecord) (issueops.IssueOpsReadiness, implementation.LocalChangeObservation) {
	return issueOpsObservedPRReadiness(record, nil)
}

func IssueOpsStrictPRReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	ready, _ := issueOpsObservedPRReadiness(record, fetchIssueOpsUpstream)
	return ready
}

// issueOpsUpstreamFetch는 strict readiness가 upstream 동기화를 판정하기 전에
// 실행한 `git fetch`의 결과다.
type issueOpsUpstreamFetch struct {
	gitRoot string
	failed  bool
	stderr  string
}

// issueOpsUpstreamFetcher는 strict 판정이 fetch 결과를 얻는 방법이다. fetch는
// 네트워크 호출이라 state root 전역 span 안에서 실행하지 않는다. span 안의
// 판정은 prefetchIssueOpsUpstream이 span 밖에서 끝낸 결과를 받는다.
type issueOpsUpstreamFetcher func(gitRoot string) issueOpsUpstreamFetch

func fetchIssueOpsUpstream(gitRoot string) issueOpsUpstreamFetch {
	code, _, stderr := GitCmd(gitRoot, "fetch", "--quiet")
	return issueOpsUpstreamFetch{gitRoot: gitRoot, failed: code != 0, stderr: strings.TrimSpace(stderr)}
}

// prefetchIssueOpsUpstream은 strict 판정이 fetch할 조건(git root와 upstream)을
// span 밖에서 확인해 미리 fetch한다. 돌려주는 fetcher는 같은 git root를 물을
// 때만 그 결과를 주고, 그 밖에는 fetch하지 않은 채 실패로 판정하게 한다.
func prefetchIssueOpsUpstream(record issueops.IssueOpsRecord) issueOpsUpstreamFetcher {
	gitRoot := issueOpsStrictGitRoot(record)
	fetched := issueOpsUpstreamFetch{gitRoot: gitRoot, failed: true, stderr: "upstream was not fetched before this readiness check; retry the command"}
	if gitRoot != "" && strings.TrimSpace(GitOut(gitRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")) != "" {
		fetched = fetchIssueOpsUpstream(gitRoot)
	}
	return func(root string) issueOpsUpstreamFetch {
		if root != fetched.gitRoot {
			return issueOpsUpstreamFetch{gitRoot: root, failed: true, stderr: "IssueOps worktree changed after the upstream fetch; retry the command"}
		}
		return fetched
	}
}

// issueOpsObservedPRReadiness는 local과 strict 두 표면의 유일한 본체다.
// fetchUpstream이 nil이면 fetch와 동기화 판정을 건너뛰고, 같은 변경 관측을
// schema 판정에도 쓴다. strict schema 판정은 fetch 뒤 경로를 다시 읽어 원격
// ref 갱신을 반영한다.
func issueOpsObservedPRReadiness(record issueops.IssueOpsRecord, fetchUpstream issueOpsUpstreamFetcher) (issueops.IssueOpsReadiness, implementation.LocalChangeObservation) {
	syncUpstream := fetchUpstream != nil
	ready := IssueOpsPRReadiness(record)
	ready.Strict = syncUpstream
	missing := append([]string{}, ready.Missing...)
	currentHead := ""
	currentFingerprint := ""
	changeObservation := implementation.LocalChangeObservation{}

	gitRoot := issueOpsStrictGitRoot(record)
	facts := issueopsdomain.PRGitFacts{RootAvailable: gitRoot != "", SyncUpstream: syncUpstream}
	if facts.RootAvailable {
		code, out, _ := GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree")
		facts.GitWorktree = code == 0 && strings.TrimSpace(out) == "true"
	}
	if facts.GitWorktree {
		currentHead = issueOpsCurrentHead(record)
		changeObservation = implementation.ObserveLocalChangesAt(record, gitRoot)
		currentFingerprint = changeObservation.Fingerprint
		facts.FingerprintVerified = changeObservation.Verified
		facts.CurrentBranch = strings.TrimSpace(GitOut(gitRoot, "branch", "--show-current"))
		facts.WorktreeClean = strings.TrimSpace(GitOut(gitRoot, "status", "--porcelain=v1")) == ""
		// base drift는 경고다. missing에 넣지 않으므로 PR 게이트 정책은 그대로다.
		// fetch하지 않고 로컬 tracking ref만 본다 — 진실은 `execution sync-base
		// --preview`가 fetch해서 확인한다.
		if base := preparedBaseRef(record); base != "" {
			remoteRef := "origin/" + base
			if code, _, _ := GitCmd(gitRoot, "rev-parse", "--verify", "--end-of-options", remoteRef+"^{commit}"); code == 0 {
				if code, _, _ := GitCmd(gitRoot, "merge-base", "--is-ancestor", remoteRef, "HEAD"); code != 0 {
					facts.BaseRemoteRef, facts.BaseAdvanced = remoteRef, true
				}
			}
		}
		facts.Upstream = strings.TrimSpace(GitOut(gitRoot, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"))
		if facts.Upstream != "" && syncUpstream {
			fetched := fetchUpstream(gitRoot)
			facts.FetchFailed, facts.FetchStderr = fetched.failed, fetched.stderr
			facts.UpstreamCounts = GitOut(gitRoot, "rev-list", "--left-right", "--count", "HEAD...@{u}")
		}
	}
	gitMissing, warnings := issueopsdomain.PRGitReadiness(record, facts)
	missing = append(missing, gitMissing...)
	// strict는 :12에서 IssueOpsPRReadiness를 포함하므로 미기록/비-pass 미싱은
	// 이미 들어 있다 — 여기서는 fingerprint가 필요한 stale 판정만 추가한다.
	if reviewMissing := implementationReviewMissing(record, currentFingerprint); strings.HasSuffix(reviewMissing, "_stale") {
		missing = append(missing, reviewMissing)
	}
	if docsMissing := projectDocsReviewMissing(record, currentFingerprint); strings.HasSuffix(docsMissing, "_stale") {
		missing = append(missing, docsMissing)
	}
	// local은 검증된 관측을 공유한다. strict는 fetch가 fallback ref를 바꿀 수
	// 있으므로 기존 순서대로 fetch 뒤 변경 경로를 새로 관측한다.
	schemaMissing := ""
	if syncUpstream {
		schemaMissing = schemaEvidenceMissing(record, currentFingerprint)
	} else if record.Execution != nil {
		schemaMissing = schemaEvidenceMissingForPaths(record, changeObservation.Paths, currentFingerprint)
	}
	if schemaMissing != "" {
		missing = append(missing, schemaMissing)
	}
	missing = append(missing, issueopsdomain.AISlopCleanFingerprintReadinessMissing(record, currentFingerprint)...)

	if path := strings.TrimSpace(record.PlanPath); path != "" && !issueOpsPlanPathExists(gitRoot, path) {
		missing = append(missing, "plan_exists")
	}
	if !issueOpsPlanInLinkedWorktree(record) {
		missing = append(missing, "plan_in_worktree")
	}
	if path := strings.TrimSpace(record.WorktreePath); path == "" {
		missing = append(missing, "worktree_path")
	} else if !issueOpsWorktreePathValid(path) {
		missing = append(missing, "worktree_exists")
	}
	missing = append(missing, issueopsdomain.TargetBranchMatchMissing(record)...)

	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Warnings = warnings
	ready.AISlopCleanHead = record.AISlopCleanHead
	ready.CurrentHead = currentHead
	ready.AISlopCleanFingerprint = record.AISlopCleanFingerprint
	ready.CurrentFingerprint = currentFingerprint
	ready.Ready = len(ready.Missing) == 0
	return ready, changeObservation
}

func issueOpsStrictPRReadinessWithState(stateRoot string, record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithStateUsing(stateRoot, record, fetchIssueOpsUpstream)
}

func issueOpsStrictPRReadinessWithStateUsing(stateRoot string, record issueops.IssueOpsRecord, fetchUpstream issueOpsUpstreamFetcher) issueops.IssueOpsReadiness {
	ready, _ := issueOpsObservedPRReadiness(record, fetchUpstream)
	childMissing, childWarnings := issueOpsChildPRGateMissing(stateRoot, record)
	if len(childMissing) == 0 && len(childWarnings) == 0 {
		return ready
	}
	ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), childMissing...))
	ready.Warnings = append(ready.Warnings, childWarnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func IssueOpsStrictPRReadinessWithState(stateRoot string, record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsStrictPRReadinessWithState(stateRoot, record)
}
