// Package issueopscycle 패키지는 IssueOps 사이클의 phase 전이와 준비 상태(readiness) 판정을 조율하는 응용 계층이다.
// 도메인 규칙은 domain 계층의 판정 함수에, 저장소·git 관찰은 port 인터페이스에 두고 이 패키지가 둘을 이어
// phase 진입 검증, 봉인 갱신, PR 게이트, 변경 권한 검사를 수행한다.
package issueopscycle

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
	review "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

// PhaseService 타입은 사이클 phase 전이를 잠금 안에서 수행한다. 진입 준비 상태를 검증하고 봉인(head, fingerprint)을 찍어 기록과 추적 자료를 갱신한다.
type PhaseService struct {
	Store          cycleport.PhaseStore
	WriteMaterials func(issueops.Record) issueops.TrackedMaterials
	Readiness      Readiness
	Transitions    cycleport.PhaseTransitionObservations
}

func (s PhaseService) touchWrite(root string, record issueops.Record) (issueops.Record, error) {
	record.UpdatedAt = s.Store.Now()
	return s.Store.Write(root, record)
}

// Refresh 메서드는 ai-slop-clean 준비 상태를 확인한 뒤 현재 head와 지문으로 봉인을 다시 찍어 기록한다.
func (s PhaseService) Refresh(root string, record issueops.Record) (issueops.Record, error) {
	return refreshAISlopClean(cycleport.AISlopCleanRefreshStore{Readiness: s.Readiness.AISlopClean, Now: s.Store.Now, Head: s.Transitions.Head, Fingerprint: s.Transitions.Fingerprint, TouchWrite: s.touchWrite}, root, record)
}

// ProblemReadiness 함수는 기록의 문제 정의(intent)에서 빠진 항목으로 준비 상태를 만든다.
func ProblemReadiness(record issueops.Record) issueops.Readiness {
	return readinessFromMissing(record, issueopsdomain.IntentMissing(record))
}

// GrillReadiness 함수는 grill 단계에서 빠진 항목으로 준비 상태를 만든다.
func GrillReadiness(record issueops.Record) issueops.Readiness {
	return readinessFromMissing(record, issueopsdomain.GrillReadinessMissing(record))
}

// AdvanceReport 메서드는 기록을 to phase로 전이하고 갱신된 기록과 추적 자료 보고를 돌려준다. pr 진입이면 잠금 밖에서 upstream fetch를 미리 하고, 잠금 안에서 권한과 진입 준비 상태를 검증한다.
func (s PhaseService) AdvanceReport(stateRoot, id, to string) (issueops.Record, issueops.TrackedMaterials, error) {
	var materials issueops.TrackedMaterials
	upstream, err := s.prefetchForPhase(stateRoot, id, to)
	if err != nil {
		return issueops.Record{OK: false}, materials, err
	}
	var rec issueops.Record
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
	if issueops.Phase(strings.TrimSpace(to)) != issueops.PhasePR {
		return nil, nil
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return nil, err
	}
	if err := s.Store.ValidateMutation(record); err != nil {
		return nil, err
	}
	return s.Readiness.prefetchUpstream(record), nil
}

func (s PhaseService) advanceLocked(stateRoot, id, to string, upstream func(string) review.UpstreamFetch) (issueops.Record, issueops.TrackedMaterials, error) {
	var materials issueops.TrackedMaterials
	phase := issueops.Phase(strings.TrimSpace(to))
	if !issueopsdomain.KnownIssueOpsPhase(phase) {
		// Probe: `--to verify` (a next stage, not a phase) and a missing --to
		// named nothing to use instead.
		names := make([]string, 0, len(issueops.Phases))
		for _, known := range issueops.Phases {
			names = append(names, string(known))
		}
		return issueops.Record{OK: false}, materials, fmt.Errorf("unknown issueops phase %q (--to wants %s; next's stage.key is not a phase, follow its next_command)", to, strings.Join(names, "|"))
	}
	record, err := s.Store.Read(stateRoot, id)
	if err != nil {
		return record, materials, err
	}
	if record.Phase == phase {
		record, err = s.resealSamePhase(stateRoot, record, phase, upstream)
		return record, materials, err
	}
	if issueopsdomain.ShouldRefreshAISlopClean(record, phase) {
		record, err = s.Refresh(stateRoot, record)
		if err == nil {
			// Agents read the unchanged phase in the ack as a no-op.
			materials.Warnings = append(materials.Warnings, fmt.Sprintf(
				"done: the ai-slop-clean seal is re-taken at the current head and the phase stays %s by design; issueops next names the reviews that are now stale", record.Phase))
		}
		return record, materials, err
	}
	if err := validatePhaseEntry(s.entryReadiness(stateRoot, upstream), record, phase); err != nil {
		return issueops.Record{OK: false}, materials, err
	}
	if (phase == issueops.PhaseImplement || phase == issueops.PhaseAISlopClean) && s.WriteMaterials != nil {
		materials = s.WriteMaterials(record)
	}
	record = applyPhaseTransition(cycleport.PhaseTransitionObservations{
		Now:         s.Store.Now,
		Head:        s.Transitions.Head,
		Fingerprint: s.Transitions.Fingerprint,
	}, record, phase)
	record, err = s.touchWrite(stateRoot, record)
	return record, materials, err
}

// resealSamePhase handles `--to` naming the phase the record is already in:
// only ai-slop-clean and pr re-take their seal, every other phase is a no-op.
func (s PhaseService) resealSamePhase(stateRoot string, record issueops.Record, phase issueops.Phase, upstream func(string) review.UpstreamFetch) (issueops.Record, error) {
	switch phase {
	case issueops.PhaseAISlopClean:
		return s.Refresh(stateRoot, record)
	case issueops.PhasePR:
		// pr 안에서 코드를 고치고 정리·문서·검증 봉인을 새 head로 다시 받은 뒤의 재봉인이다.
		// pr 진입과 같은 strict 판정을 통과해야 completion이 비교할 pr_head가 바뀐다.
		if err := validatePhaseEntry(s.entryReadiness(stateRoot, upstream), record, phase); err != nil {
			return issueops.Record{OK: false}, err
		}
		record.PRHead = s.Transitions.Head(record)
		return s.touchWrite(stateRoot, record)
	}
	return record, nil
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
		StrictPR: func(record issueops.Record) issueops.Readiness {
			if upstream == nil {
				upstream = func(gitRoot string) review.UpstreamFetch {
					return review.UpstreamFetch{Root: gitRoot, Failed: true, Stderr: "upstream was not fetched before the pr transition; retry the command"}
				}
			}
			return s.Readiness.strictPRWithFetch(stateRoot, record, upstream)
		},
		RemoteArtifactMissing: issueopsdomain.RemoteArtifactMissing,
	}
}
