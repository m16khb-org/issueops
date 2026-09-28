package issueops

import (
	"fmt"
	"strings"
	"time"

	"context"

	"issueops/internal/adapter/issueops/implementation"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func knownIssueOpsPhase(phase issueops.IssueOpsPhase) bool {
	return issueopsdomain.KnownIssueOpsPhase(phase)
}

func issueOpsPhaseRank(phase issueops.IssueOpsPhase) int {
	return issueopsdomain.IssueOpsPhaseRank(phase)
}

func AdvanceIssueOpsPhase(stateRoot, id, to string) (issueops.IssueOpsRecord, error) {
	return advanceIssueOpsPhaseWithActor(stateRoot, id, to, nil)
}

func AdvanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return advanceIssueOpsPhaseWithActor(stateRoot, id, to, &actor)
}

func advanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	upstream, err := prefetchIssueOpsUpstreamForPhase(stateRoot, id, to, actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	var rec issueops.IssueOpsRecord
	err = withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := validateExecutionMutation(record, actor); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = advanceIssueOpsPhaseLocked(stateRoot, id, to, upstream)
		return e
	})
	return rec, err
}

// prefetchIssueOpsUpstreamForPhase는 pr 진입일 때만 strict 판정이 쓸 fetch를
// span 밖에서 끝낸다. 다른 전이는 fetch하지 않는다. 거부될 호출자를 위해 fetch하지
// 않도록 권한을 먼저 보고, 권한 판정은 span 안에서 다시 한다.
func prefetchIssueOpsUpstreamForPhase(stateRoot, id, to string, actor *IssueOpsActor) (issueOpsUpstreamFetcher, error) {
	if issueops.IssueOpsPhase(strings.TrimSpace(to)) != IssueOpsPhasePR {
		return nil, nil
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return nil, err
	}
	if record.Phase == IssueOpsPhasePR {
		return nil, nil
	}
	if err := validateExecutionMutation(record, actor); err != nil {
		return nil, err
	}
	return prefetchIssueOpsUpstream(record), nil
}

func advanceIssueOpsPhaseLocked(stateRoot, id, to string, upstream issueOpsUpstreamFetcher) (issueops.IssueOpsRecord, error) {
	phase := issueops.IssueOpsPhase(strings.TrimSpace(to))
	if !knownIssueOpsPhase(phase) {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == phase {
		if phase == IssueOpsPhaseAISlopClean {
			return refreshIssueOpsAISlopClean(stateRoot, record)
		}
		return record, nil
	}
	if issueopsdomain.ShouldRefreshAISlopClean(record, phase) {
		return refreshIssueOpsAISlopClean(stateRoot, record)
	}
	if err := cycleapp.ValidatePhaseEntry(phaseEntryReadiness(stateRoot, upstream), record, phase); err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	record = cycleapp.ApplyPhaseTransition(cycleport.PhaseTransitionObservations{
		Now:         func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		Head:        issueOpsCurrentHead,
		Fingerprint: implementation.ChangeFingerprint,
	}, record, phase)
	return touchAndWriteIssueOps(stateRoot, record)
}

// validateIssueOpsPhaseTransition은 span 안에서 불린다. pr 진입 판정은 upstream
// fetch 결과가 필요하지만 fetch 자체는 호출자가 span 밖에서 끝낸 것만 쓴다.
// 결과가 없으면 fetch하지 않은 것으로 보고 upstream_fetch로 거부한다.
func phaseEntryReadiness(stateRoot string, upstream issueOpsUpstreamFetcher) cycleport.PhaseEntryReadiness {
	return cycleport.PhaseEntryReadiness{
		Problem:       IssueOpsProblemReadiness,
		Grill:         IssueOpsGrillReadiness,
		Plan:          IssueOpsPlanReadiness,
		Compatibility: IssueOpsCompatibilityReviewReadiness,
		Implement:     IssueOpsImplementationReadiness,
		AISlopClean:   IssueOpsAISlopCleanReadiness,
		StrictPR: func(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
			if upstream == nil {
				upstream = func(gitRoot string) issueOpsUpstreamFetch {
					return issueOpsUpstreamFetch{gitRoot: gitRoot, failed: true, stderr: "upstream was not fetched before the pr transition; retry the command"}
				}
			}
			return issueOpsStrictPRReadinessWithStateUsing(stateRoot, record, upstream)
		},
		RemoteArtifactMissing: issueopsdomain.RemoteArtifactMissing,
	}
}
