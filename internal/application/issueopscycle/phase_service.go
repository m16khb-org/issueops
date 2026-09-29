package issueopscycle

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

type PhaseService struct {
	Store       cycleport.PhaseStore
	Readiness   Readiness
	Transitions cycleport.PhaseTransitionObservations
}

func (s PhaseService) touchWrite(root string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	record.UpdatedAt = s.Store.Now()
	return s.Store.Write(root, record)
}
func (s PhaseService) Refresh(root string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	return RefreshAISlopClean(cycleport.AISlopCleanRefreshStore{Readiness: s.Readiness.AISlopClean, Now: s.Store.Now, Head: s.Transitions.Head, Fingerprint: s.Transitions.Fingerprint, TouchWrite: s.touchWrite}, root, record)
}
func ProblemReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return ReadinessFromMissing(record, issueopsdomain.IntentMissing(record))
}
func GrillReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return ReadinessFromMissing(record, issueopsdomain.GrillReadinessMissing(record))
}

func (s PhaseService) Advance(stateRoot, id, to string) (issueops.IssueOpsRecord, error) {
	upstream, err := s.prefetchForPhase(stateRoot, id, to)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	var rec issueops.IssueOpsRecord
	err = s.Store.WithLock(stateRoot, id, func() error {
		record, readErr := s.Store.Read(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if actorErr := s.Store.ValidateMutation(record); actorErr != nil {
			return actorErr
		}
		var e error
		rec, e = s.advanceLocked(stateRoot, id, to, upstream)
		return e
	})
	return rec, err
}

// prefetchForPhase는 pr 진입일 때만 strict 판정이 쓸 fetch를
// span 밖에서 끝낸다. 다른 전이는 fetch하지 않는다. 거부될 호출자를 위해 fetch하지
// 않도록 권한을 먼저 보고, 권한 판정은 span 안에서 다시 한다.
func (s PhaseService) prefetchForPhase(stateRoot, id, to string) (func(string) review.UpstreamFetch, error) {
	if issueops.IssueOpsPhase(strings.TrimSpace(to)) != issueops.IssueOpsPhasePR {
		return nil, nil
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return nil, err
	}
	if record.Phase == issueops.IssueOpsPhasePR {
		return nil, nil
	}
	if err := s.Store.ValidateMutation(record); err != nil {
		return nil, err
	}
	return s.Readiness.PrefetchUpstream(record), nil
}

func (s PhaseService) advanceLocked(stateRoot, id, to string, upstream func(string) review.UpstreamFetch) (issueops.IssueOpsRecord, error) {
	phase := issueops.IssueOpsPhase(strings.TrimSpace(to))
	if !issueopsdomain.KnownIssueOpsPhase(phase) {
		return issueops.IssueOpsRecord{OK: false}, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	if record.Phase == phase {
		if phase == issueops.IssueOpsPhaseAISlopClean {
			return s.Refresh(stateRoot, record)
		}
		return record, nil
	}
	if issueopsdomain.ShouldRefreshAISlopClean(record, phase) {
		return s.Refresh(stateRoot, record)
	}
	if err := ValidatePhaseEntry(s.entryReadiness(stateRoot, upstream), record, phase); err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	record = ApplyPhaseTransition(cycleport.PhaseTransitionObservations{
		Now:         s.Store.Now,
		Head:        s.Transitions.Head,
		Fingerprint: s.Transitions.Fingerprint,
	}, record, phase)
	return s.touchWrite(stateRoot, record)
}

// entryReadiness는 span 안에서 불린다. pr 진입 판정은 upstream
// fetch 결과가 필요하지만 fetch 자체는 호출자가 span 밖에서 끝낸 것만 쓴다.
// 결과가 없으면 fetch하지 않은 것으로 보고 upstream_fetch로 거부한다.
func (s PhaseService) entryReadiness(stateRoot string, upstream func(string) review.UpstreamFetch) cycleport.PhaseEntryReadiness {
	return cycleport.PhaseEntryReadiness{
		Problem:       ProblemReadiness,
		Grill:         GrillReadiness,
		Plan:          s.Readiness.Plan,
		Compatibility: s.Readiness.Compatibility,
		Implement:     s.Readiness.Implementation,
		AISlopClean:   s.Readiness.AISlopClean,
		StrictPR: func(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
			if upstream == nil {
				upstream = func(gitRoot string) review.UpstreamFetch {
					return review.UpstreamFetch{Root: gitRoot, Failed: true, Stderr: "upstream was not fetched before the pr transition; retry the command"}
				}
			}
			return s.Readiness.StrictPRWithFetch(stateRoot, record, upstream)
		},
		RemoteArtifactMissing: issueopsdomain.RemoteArtifactMissing,
	}
}
