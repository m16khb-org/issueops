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
	Store          cycleport.PhaseStore
	WriteMaterials func(issueops.IssueOpsRecord) issueops.IssueOpsTrackedMaterials
	Readiness      Readiness
	Transitions    cycleport.PhaseTransitionObservations
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

func (s PhaseService) AdvanceReport(stateRoot, id, to string) (issueops.IssueOpsRecord, issueops.IssueOpsTrackedMaterials, error) {
	var materials issueops.IssueOpsTrackedMaterials
	upstream, err := s.prefetchForPhase(stateRoot, id, to)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, materials, err
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
		rec, materials, e = s.advanceLocked(stateRoot, id, to, upstream)
		return e
	})
	return rec, materials, err
}

// prefetchForPhase는 pr 진입일 때만 strict 판정이 쓸 fetch를
// span 밖에서 끝낸다. 이미 pr이면 다시 봉인하는 호출이므로 같은 strict 판정을 위해 fetch한다.
// 다른 전이는 fetch하지 않는다. 거부될 호출자를 위해 fetch하지
// 않도록 권한을 먼저 보고, 권한 판정은 span 안에서 다시 한다.
func (s PhaseService) prefetchForPhase(stateRoot, id, to string) (func(string) review.UpstreamFetch, error) {
	if issueops.IssueOpsPhase(strings.TrimSpace(to)) != issueops.IssueOpsPhasePR {
		return nil, nil
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return nil, err
	}
	if err := s.Store.ValidateMutation(record); err != nil {
		return nil, err
	}
	return s.Readiness.PrefetchUpstream(record), nil
}

func (s PhaseService) advanceLocked(stateRoot, id, to string, upstream func(string) review.UpstreamFetch) (issueops.IssueOpsRecord, issueops.IssueOpsTrackedMaterials, error) {
	var materials issueops.IssueOpsTrackedMaterials
	phase := issueops.IssueOpsPhase(strings.TrimSpace(to))
	if !issueopsdomain.KnownIssueOpsPhase(phase) {
		return issueops.IssueOpsRecord{OK: false}, materials, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return record, materials, err
	}
	if record.Phase == phase {
		switch phase {
		case issueops.IssueOpsPhaseAISlopClean:
			record, err = s.Refresh(stateRoot, record)
			return record, materials, err
		case issueops.IssueOpsPhasePR:
			// pr 안에서 코드를 고치고 정리·문서·검증 봉인을 새 head로 다시 받은 뒤의 재봉인이다.
			// pr 진입과 같은 strict 판정을 통과해야 completion이 비교할 pr_head가 바뀐다.
			if err := ValidatePhaseEntry(s.entryReadiness(stateRoot, upstream), record, phase); err != nil {
				return issueops.IssueOpsRecord{OK: false}, materials, err
			}
			record.PRHead = s.Transitions.Head(record)
			record, err = s.touchWrite(stateRoot, record)
			return record, materials, err
		}
		return record, materials, nil
	}
	if issueopsdomain.ShouldRefreshAISlopClean(record, phase) {
		record, err = s.Refresh(stateRoot, record)
		if err == nil {
			// Agents read the unchanged phase in the ack as a no-op.
			materials.Warnings = append(materials.Warnings, fmt.Sprintf(
				"phase stays %s: from a later phase this re-takes the ai-slop-clean seal at the current head; issueops next names the reviews that are now stale", record.Phase))
		}
		return record, materials, err
	}
	if err := ValidatePhaseEntry(s.entryReadiness(stateRoot, upstream), record, phase); err != nil {
		return issueops.IssueOpsRecord{OK: false}, materials, err
	}
	if (phase == issueops.IssueOpsPhaseImplement || phase == issueops.IssueOpsPhaseAISlopClean) && s.WriteMaterials != nil {
		materials = s.WriteMaterials(record)
	}
	record = ApplyPhaseTransition(cycleport.PhaseTransitionObservations{
		Now:         s.Store.Now,
		Head:        s.Transitions.Head,
		Fingerprint: s.Transitions.Fingerprint,
	}, record, phase)
	record, err = s.touchWrite(stateRoot, record)
	return record, materials, err
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
