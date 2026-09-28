package issueops

import (
	"context"
	"fmt"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/active"
	"issueops/internal/adapter/issueops/cleanupchildren"
	"issueops/internal/adapter/issueops/cleanupstatus"
	"issueops/internal/adapter/issueops/compatibilityreview"
	"issueops/internal/adapter/issueops/devilsadvocate"
	"issueops/internal/adapter/issueops/intentdesign"
	branchapp "issueops/internal/application/issueopsbranch"
	reviewapp "issueops/internal/application/issueopsreview"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsintent"
	"issueops/internal/domain/repoidentity"
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

// LastActiveAt returns the latest durable lifecycle timestamp.
func LastActiveAt(record issueops.IssueOpsRecord) string {
	if strings.TrimSpace(record.UpdatedAt) != "" {
		return record.UpdatedAt
	}
	return record.CreatedAt
}
