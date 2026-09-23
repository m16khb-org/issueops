package issueops

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"issueops/internal/adapter/issueops/active"
	"issueops/internal/adapter/issueops/artifactverify"
	"issueops/internal/adapter/issueops/branchprepare"
	"issueops/internal/adapter/issueops/cleanupchildren"
	"issueops/internal/adapter/issueops/cleanupstatus"
	"issueops/internal/adapter/issueops/compatibilityreview"
	"issueops/internal/adapter/issueops/devilsadvocate"
	"issueops/internal/adapter/issueops/intentdesign"
	"issueops/internal/adapter/issueops/linking"
	"issueops/internal/adapter/issueops/start"
	"issueops/internal/contract/issueops"
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

func VerifyIssueOpsRemoteArtifactWithActor(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return verifyIssueOpsRemoteArtifact(stateRoot, id, req, &actor)
}

func verifyIssueOpsRemoteArtifact(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validatePostTransferMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = artifactverify.Verify(issueOpsArtifactStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func ValidateIssueOpsRemoteArtifactVerification(stateRoot, id string, req issueops.IssueOpsRemoteArtifactVerificationRequest) (issueops.IssueOpsRecord, error) {
	var rec issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		var e error
		rec, e = artifactverify.Validate(issueOpsArtifactStore(), stateRoot, id, req)
		return e
	})
	return rec, err
}

func issueOpsArtifactStore() artifactverify.Store {
	return artifactverify.Store{
		Read:       ReadIssueOps,
		TouchWrite: touchAndWriteIssueOps,
	}
}

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

// BranchRetargetDeps는 branch retarget의 외부 관측 주입점이다. artifact target
// readback은 provider CLI 표면(cmd 계층)에 있으므로 여기서 주입받는다.
type BranchRetargetDeps struct {
	ObserveArtifactTargetBranch func(artifact issueops.IssueOpsRemoteArtifactVerification) (string, error)
}

func RetargetIssueOpsBranchWithActor(stateRoot, id string, req issueops.IssueOpsBranchRetargetRequest, actor IssueOpsActor, deps BranchRetargetDeps) (issueops.IssueOpsRecord, error) {
	// provider readback과 origin 관측은 원격 호출이다. state root 전역 span 안에서
	// 기다리면 다른 모든 사이클의 쓰기가 멈추므로 span 밖에서 한 번 관측하고,
	// span 안의 Retarget은 같은 artifact와 같은 repo·branch를 물을 때만 그 결과를
	// 쓴다.
	observed, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	// 거부될 호출자를 위해 원격을 조회하지 않는다. 권한 판정은 span 안에서 다시 한다.
	if actorErr := validateRetargetMutation(observed, &actor); actorErr != nil {
		return issueops.IssueOpsRecord{OK: false}, actorErr
	}
	observation := observeBranchRetarget(observed, strings.TrimSpace(req.BaseBranch), deps)
	var rec issueops.IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateRetargetMutation(record, &actor); actorErr != nil {
			return actorErr
		}
		store := issueOpsBranchPrepareStore()
		if deps.ObserveArtifactTargetBranch != nil {
			store.ObserveArtifactTargetBranch = observation.artifactTargetBranch
			store.RemoteBranchPresent = observation.remoteBranchPresent
		}
		var e error
		rec, e = branchprepare.Retarget(store, stateRoot, id, req)
		return e
	})
	return rec, err
}

var errBranchRetargetObservationStale = errors.New("IssueOps record changed while the retarget was observed; retry the command")

// branchRetargetObservation은 span 밖에서 끝낸 재타깃 관측이다.
type branchRetargetObservation struct {
	artifact      *issueops.IssueOpsRemoteArtifactVerification
	target        string
	targetErr     error
	repo, branch  string
	present       bool
	presentErr    error
	presentStated bool
}

// observeBranchRetarget은 Retarget이 묻는 순서 그대로 관측한다. artifact target이
// 요청한 base가 아니면 Retarget이 거기서 거부하므로 origin은 조회하지 않는다.
func observeBranchRetarget(record issueops.IssueOpsRecord, baseBranch string, deps BranchRetargetDeps) branchRetargetObservation {
	var observation branchRetargetObservation
	if deps.ObserveArtifactTargetBranch == nil || record.RemoteArtifact == nil || baseBranch == "" {
		return observation
	}
	artifact := *record.RemoteArtifact
	observation.artifact = &artifact
	observation.target, observation.targetErr = deps.ObserveArtifactTargetBranch(artifact)
	if observation.targetErr != nil || strings.TrimSpace(observation.target) != baseBranch {
		return observation
	}
	observation.repo, observation.branch = record.Repo, baseBranch
	observation.present, observation.presentErr = issueOpsOriginBranchPresent(record.Repo, baseBranch)
	observation.presentStated = true
	return observation
}

func (observation branchRetargetObservation) artifactTargetBranch(artifact issueops.IssueOpsRemoteArtifactVerification) (string, error) {
	if observation.artifact == nil || !reflect.DeepEqual(*observation.artifact, artifact) {
		return "", errBranchRetargetObservationStale
	}
	return observation.target, observation.targetErr
}

func (observation branchRetargetObservation) remoteBranchPresent(repo, branch string) (bool, error) {
	if !observation.presentStated || observation.repo != repo || observation.branch != branch {
		return false, errBranchRetargetObservationStale
	}
	return observation.present, observation.presentErr
}

// issueOpsOriginBranchPresent는 origin에 branch가 있는지 본다. 네트워크 호출이라
// span 밖에서만 부른다. 사용자의 SSH·자격 증명 설정(core.sshCommand 등)을 그대로
// 쓰도록 주입된 GitCmd로 실행한다.
func issueOpsOriginBranchPresent(repo, branch string) (bool, error) {
	code, stdout, stderr := GitCmd(repo, "ls-remote", "--heads", "origin", "refs/heads/"+strings.TrimSpace(branch))
	if code != 0 {
		return false, fmt.Errorf("git ls-remote failed: %s", strings.TrimSpace(stderr))
	}
	return len(strings.Fields(strings.TrimSpace(stdout))) > 0, nil
}

func validateIssueOpsIssueBranch(branch string) error {
	return branchprepare.ValidateBranch(branch)
}

func issueOpsBranchPrepareStore() branchprepare.Store {
	return branchprepare.Store{
		Read:             ReadIssueOps,
		TouchWrite:       touchAndWriteIssueOps,
		ValidateIssueURL: linking.ValidateIssueURL,
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

// issueOpsStartLockID computes the lock id used by StartIssueOps. It must
// mirror start.Start's record-id derivation exactly: trim repo+branch and
// abs-normalize the repo (filepath.Abs) before hashing, so that a relative and
// the equivalent absolute repo path take the SAME lock and serialize on the
// SAME record. newIssueOpsID does no repository normalization, so hashing the
// raw repo here would let source-checkout and linked-worktree starts hold
// different locks while read-modify-writing one record (lost-update TOCTOU).
func issueOpsStartLockID(repo, branch string) string {
	repo = normalizeIssueOpsRepo(repo)
	return newIssueOpsID(repo, strings.TrimSpace(branch))
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
	id, err := issueOpsStartRecordID(req)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	var rec issueops.IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		store := issueOpsStartStore()
		if req.New {
			store.NewID = func(string, string) string { return id }
		}
		var e error
		rec, e = start.Start(store, stateRoot, req)
		return e
	})
	return rec, err
}

func issueOpsStartRecordID(req issueops.IssueOpsStartRequest) (string, error) {
	if !req.New {
		return issueOpsStartLockID(req.Repo, req.Branch), nil
	}
	if strings.TrimSpace(req.Branch) != "" {
		return "", fmt.Errorf("a new issueops cycle requires a branchless start")
	}
	return newIndependentIssueOpsID(normalizeIssueOpsRepo(req.Repo))
}

func issueOpsStartStore() start.Store {
	return start.Store{
		Read:           ReadIssueOps,
		Write:          writeIssueOps,
		NewID:          newIssueOpsID,
		ValidateBranch: validateIssueOpsIssueBranch,
		NormalizeRepo:  normalizeIssueOpsRepo,
	}
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
	return intentdesign.CleanTextValues(values)
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
		rec, e = devilsadvocate.Record(devilsadvocate.Store{Read: ReadIssueOps, TouchWrite: touchAndWriteIssueOps, PlanDigest: issueOpsReviewedPlanDigest}, stateRoot, id, req)
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
		BranchEvidenceMissing:  issueOpsBranchEvidenceMissing,
		DesignReviewMissing:    issueOpsDesignReviewMissing,
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
