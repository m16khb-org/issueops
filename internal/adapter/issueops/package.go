package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/active"
	"issueops/internal/adapter/issueops/branchprepare"
	"issueops/internal/adapter/issueops/cleanupchildren"
	"issueops/internal/adapter/issueops/cleanupstatus"
	"issueops/internal/adapter/issueops/compatibilityreview"
	"issueops/internal/adapter/issueops/devilsadvocate"
	"issueops/internal/adapter/issueops/intentdesign"
	"issueops/internal/adapter/issueops/linking"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsintent"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/repoidentity"
	"issueops/internal/domain/stringlist"
	"issueops/internal/port"
)

const (
	IssueOpsCurrentSchemaVersion     = issueops.IssueOpsCurrentSchemaVersion
	IssueOpsPhaseCompatibilityReview = issueops.IssueOpsPhaseCompatibilityReview
	IssueOpsPhaseFeedback            = issueops.IssueOpsPhaseFeedback
)

var IssueOpsPhases = issueops.IssueOpsPhases

func issueOpsActiveStore() active.Store {
	return active.Store{
		StateRoot: IssueOpsStateRoot,
		// Hooks must still see a corrupt v1 record so they fail closed instead of
		// silently dropping the execution guard. Command paths use ReadIssueOps,
		// which validates the record before operating on it.
		Read:    readIssueOpsUnchecked,
		Scan:    ScanReadableIssueOps,
		NewID:   newIssueOpsID,
		ListIDs: ListIssueOpsIDs,
	}
}

func IssueOpsCleanupStatusForRecord(record issueops.IssueOpsRecord, req issueops.IssueOpsCleanupStatusRequest) issueops.IssueOpsCleanupStatus {
	return cleanupstatus.ForRecord(record, req)
}

func FinalizeIssueOpsCleanupStatus(status issueops.IssueOpsCleanupStatus) issueops.IssueOpsCleanupStatus {
	return cleanupstatus.Finalize(status)
}

func IssueOpsRemoteArtifactMissing(record issueops.IssueOpsRecord) []string {
	return cleanupstatus.RemoteArtifactMissing(record)
}

func CloseIssueOpsChildren(stateRoot, id string, req issueops.IssueOpsCloseChildrenRequest, provider func(string) (port.IssueProvider, error)) (issueops.IssueOpsCloseChildrenResult, error) {
	var result issueops.IssueOpsCloseChildrenResult
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		result, e = cleanupchildren.ByID(cleanupchildren.Store{
			Read:       ReadIssueOps,
			TouchWrite: touchAndWriteIssueOps,
			Provider:   provider,
		}, stateRoot, id, req)
		return e
	})
	return result, err
}

func issueOpsRemoteArtifactMissing(record issueops.IssueOpsRecord) []string {
	return cleanupstatus.RemoteArtifactMissing(record)
}

func PrepareIssueOpsBranch(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest) (issueops.IssueOpsRecord, error) {
	return prepareIssueOpsBranch(stateRoot, id, req, nil)
}

func PrepareIssueOpsBranchWithActor(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return prepareIssueOpsBranch(stateRoot, id, req, &actor)
}

func prepareIssueOpsBranch(stateRoot, id string, req issueops.IssueOpsBranchPrepareRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = branchprepare.Prepare(issueOpsBranchPrepareStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

// OriginBranchPresent는 origin에 branch가 있는지 본다. 네트워크 호출이라
// span 밖에서만 부른다. 사용자의 SSH·자격 증명 설정(core.sshCommand 등)을 그대로
// 쓰도록 주입된 GitCmd로 실행한다.
func OriginBranchPresent(repo, branch string) (bool, error) {
	code, stdout, stderr := GitCmd(repo, "ls-remote", "--heads", "origin", "refs/heads/"+strings.TrimSpace(branch))
	if code != 0 {
		return false, fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(stderr))
	}
	return len(strings.Fields(strings.TrimSpace(stdout))) > 0, nil
}

func issueOpsBranchPrepareStore() branchprepare.Store {
	return branchprepare.Store{
		Read:             ReadIssueOps,
		TouchWrite:       touchAndWriteIssueOps,
		ValidateIssueURL: issueopsdomain.ValidateIssueURL,
		ResolveBaseCommit: func(repo, revision string) (string, error) {
			code, stdout, stderr := GitCmd(
				repo,
				"rev-parse",
				"--verify",
				"--end-of-options",
				strings.TrimSpace(revision)+"^{commit}",
			)
			if code != 0 {
				return "", fmt.Errorf("git rev-parse failed: %s", strings.TrimSpace(stderr))
			}
			resolved := strings.TrimSpace(stdout)
			if resolved == "" {
				return "", fmt.Errorf("git rev-parse returned an empty commit OID")
			}
			return resolved, nil
		},
		UmbrellaForChildIssue: func(repo, childIssueURL string) (issueops.IssueOpsRecord, bool) {
			return active.UmbrellaCycleForChildIssue(issueOpsActiveStore(), repo, childIssueURL)
		},
		ObserveCodeProjectKey: func(repo, provider string) (string, error) {
			// GitCmd는 주입되는 의존이다. 없으면 관찰할 수 없다는 뜻이고,
			// resolveCodeProjectKey가 이슈 프로젝트로 되돌린다.
			if GitCmd == nil {
				return "", fmt.Errorf("git command adapter is unavailable")
			}
			code, stdout, stderr := GitCmd(repo, "remote", "get-url", "origin")
			if code != 0 {
				return "", fmt.Errorf("git remote get-url origin failed: %s", strings.TrimSpace(stderr))
			}
			return remote.ProjectKeyFromGitRemoteURL(strings.TrimSpace(stdout), provider)
		},
	}
}

func normalizeIssueOpsRepo(repo string) string {
	clean := repoidentity.SourceRoot(repo, "")
	if GitCmd == nil {
		return clean
	}
	code, commonDir, _ := GitCmd(clean, "rev-parse", "--path-format=relative", "--git-common-dir")
	if code != 0 {
		commonDir = ""
	}
	return repoidentity.SourceRoot(clean, commonDir)
}

func StartIssueOps(stateRoot string, req issueops.IssueOpsStartRequest) (issueops.IssueOpsRecord, error) {
	return (branchapp.Starter{Records: CycleRecordStore{StateRoot: stateRoot}, Identity: CycleStartIdentity{}, Now: time.Now}).Start(context.Background(), req)
}

func RecordIssueOpsIntent(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsIntent(stateRoot, id, req, nil)
}

func RecordIssueOpsIntentWithActor(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsIntent(stateRoot, id, req, &actor)
}

func recordIssueOpsIntent(stateRoot, id string, req issueops.IssueOpsIntentRecordRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordIntent(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsPlanPrep(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, nil)
}

func RecordIssueOpsPlanPrepWithActor(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsPlanPrep(stateRoot, id, req, &actor)
}

func recordIssueOpsPlanPrep(stateRoot, id string, req issueops.IssueOpsPlanPrepRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordPlanPrep(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func RecordIssueOpsDesignReview(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDesignReviewWithActor(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDesignReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDesignReview(stateRoot, id string, req issueops.IssueOpsDesignReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = intentdesign.RecordDesignReview(issueOpsIntentDesignStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func cleanIssueOpsTextValues(values []string) []string {
	return issueopsintent.CleanTextValues(values)
}

func issueOpsIntentDesignStore() intentdesign.Store {
	return intentdesign.Store{
		Read:          ReadIssueOps,
		TouchWrite:    touchAndWriteIssueOps,
		PlanReadiness: IssueOpsPlanReadiness,
	}
}

func LinkIssueOpsIssue(stateRoot, id, issueURL string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsIssue(stateRoot, id, issueURL, nil)
}

func LinkIssueOpsIssueWithActor(stateRoot, id, issueURL string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsIssue(stateRoot, id, issueURL, &actor)
}

func linkIssueOpsIssue(stateRoot, id, issueURL string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkIssue(issueOpsLinkingStore(), stateRoot, id, issueURL)
		return e
	})
	return rec, err
}

func LinkIssueOpsPlan(stateRoot, id, planPath string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, nil)
}

func LinkIssueOpsPlanWithActor(stateRoot, id, planPath string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsPlan(stateRoot, id, planPath, &actor)
}

func linkIssueOpsPlan(stateRoot, id, planPath string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validatePlanLinkMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var writeErr error
		rec, writeErr = linking.LinkPlan(issueOpsLinkingStore(), stateRoot, id, planPath)
		return writeErr
	})
	return rec, err
}

func LinkIssueOpsWorktree(stateRoot, id, worktreePath string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, nil)
}

func LinkIssueOpsWorktreeWithActor(stateRoot, id, worktreePath string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsWorktree(stateRoot, id, worktreePath, &actor)
}

func linkIssueOpsWorktree(stateRoot, id, worktreePath string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkWorktree(issueOpsLinkingStore(), stateRoot, id, worktreePath)
		return e
	})
	return rec, err
}

func RecordIssueOpsCompatibilityReview(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, nil)
}

func RecordIssueOpsCompatibilityReviewWithActor(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsCompatibilityReview(stateRoot, id, req, &actor)
}

func recordIssueOpsCompatibilityReview(stateRoot, id string, req issueops.IssueOpsCompatibilityReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = compatibilityreview.Record(issueOpsCompatibilityReviewStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func issueOpsCompatibilityReviewStore() compatibilityreview.Store {
	return compatibilityreview.Store{
		Read:       ReadIssueOps,
		TouchWrite: touchAndWriteIssueOps,
		Ready:      IssueOpsCompatibilityReviewReadiness,
		PhaseRank:  issueOpsPhaseRank,
	}
}

func RecordIssueOpsDevilsAdvocateReview(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, nil)
}

func RecordIssueOpsDevilsAdvocateReviewWithActor(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return recordIssueOpsDevilsAdvocateReview(stateRoot, id, req, &actor)
}

func recordIssueOpsDevilsAdvocateReview(stateRoot, id string, req issueops.IssueOpsDevilsAdvocateReviewRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = devilsadvocate.Record(devilsadvocate.Store{Read: ReadIssueOps, TouchWrite: touchAndWriteIssueOps, PlanDigest: reviewapp.NewPlanDigestResolver(ReviewPlanSource{}).Digest}, stateRoot, id, req)
		return e
	})
	return rec, err
}

func LinkIssueOpsChild(stateRoot, id, childURL, title string) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, nil)
}

func LinkIssueOpsChildWithActor(stateRoot, id, childURL, title string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsChild(stateRoot, id, childURL, title, &actor)
}

func linkIssueOpsChild(stateRoot, id, childURL, title string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkChild(issueOpsLinkingStore(), stateRoot, id, childURL, title)
		return e
	})
	return rec, err
}

func LinkIssueOpsRelatedWithActor(stateRoot, id, linkType, relatedURL, title string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title, &actor)
}

func linkIssueOpsRelated(stateRoot, id, linkType, relatedURL, title string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateWorkspacePreparationMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = linking.LinkRelated(issueOpsLinkingStore(), stateRoot, id, linkType, relatedURL, title)
		return e
	})
	return rec, err
}

func issueOpsLinkingStore() linking.Store {
	return linking.Store{
		Read:                   ReadIssueOps,
		TouchWrite:             touchAndWriteIssueOps,
		PlanReadiness:          IssueOpsPlanReadiness,
		PhaseRank:              issueOpsPhaseRank,
		BranchEvidenceMissing:  issueopsdomain.BranchEvidenceMissing,
		DesignReviewMissing:    cycleapp.DesignReviewMissing,
		PlanPathExists:         issueOpsPlanPathExists,
		PlanSectionsMissing:    issueOpsPlanSectionsMissing,
		PlanPathInsideWorktree: issueOpsPlanPathInsideWorktree,
		WorktreePathValid:      issueOpsWorktreePathValid,
		UniqueSorted:           stringlist.UniqueSorted,
	}
}

// LastActiveAt returns the latest durable lifecycle timestamp.
func LastActiveAt(record issueops.IssueOpsRecord) string {
	if strings.TrimSpace(record.UpdatedAt) != "" {
		return record.UpdatedAt
	}
	return record.CreatedAt
}
