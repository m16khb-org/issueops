package issueops

import (
	"fmt"
	"strings"
	"time"

	"context"

	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/contract/issueops"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
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

// AdvanceIssueOpsPhaseWithActorReport is AdvanceIssueOpsPhaseWithActor plus
// the tracked-material copies the transition wrote. The report travels in the
// return value, never in the record.
func AdvanceIssueOpsPhaseWithActorReport(stateRoot, id, to string, actor IssueOpsActor) (issueops.IssueOpsRecord, issueops.IssueOpsTrackedMaterials, error) {
	return advanceIssueOpsPhaseReport(stateRoot, id, to, &actor)
}

func advanceIssueOpsPhaseWithActor(stateRoot, id, to string, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	record, _, err := advanceIssueOpsPhaseReport(stateRoot, id, to, actor)
	return record, err
}

func advanceIssueOpsPhaseReport(stateRoot, id, to string, actor *IssueOpsActor) (issueops.IssueOpsRecord, issueops.IssueOpsTrackedMaterials, error) {
	var materials issueops.IssueOpsTrackedMaterials
	upstream, err := prefetchIssueOpsUpstreamForPhase(stateRoot, id, to, actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, materials, err
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
		rec, materials, e = advanceIssueOpsPhaseLocked(stateRoot, id, to, upstream)
		return e
	})
	return rec, materials, err
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

func advanceIssueOpsPhaseLocked(stateRoot, id, to string, upstream issueOpsUpstreamFetcher) (issueops.IssueOpsRecord, issueops.IssueOpsTrackedMaterials, error) {
	var materials issueops.IssueOpsTrackedMaterials
	phase := issueops.IssueOpsPhase(strings.TrimSpace(to))
	if !knownIssueOpsPhase(phase) {
		return issueops.IssueOpsRecord{OK: false}, materials, fmt.Errorf("unknown issueops phase %q", to)
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, materials, err
	}
	if record.Phase == phase {
		if phase == IssueOpsPhaseAISlopClean {
			record, err = refreshIssueOpsAISlopClean(stateRoot, record)
			return record, materials, err
		}
		return record, materials, nil
	}
	if shouldRefreshIssueOpsAISlopClean(record, phase) {
		record, err = refreshIssueOpsAISlopClean(stateRoot, record)
		return record, materials, err
	}
	if err := validateIssueOpsPhaseTransition(stateRoot, record, phase, upstream); err != nil {
		return issueops.IssueOpsRecord{OK: false}, materials, err
	}
	// Implement entry and implement exit are the two transitions both direct
	// and Orca cycles pass, so they refresh the tracked copies. The copies are
	// written before the transition is applied, so the change set the
	// ai-slop-clean transition seals already contains them.
	if phase == IssueOpsPhaseImplement || phase == IssueOpsPhaseAISlopClean {
		materials = writeTrackedMaterials(record)
	}
	record = applyIssueOpsPhaseTransition(record, phase)
	record, err = touchAndWriteIssueOps(stateRoot, record)
	return record, materials, err
}

// validateIssueOpsPhaseTransition은 span 안에서 불린다. pr 진입 판정은 upstream
// fetch 결과가 필요하지만 fetch 자체는 호출자가 span 밖에서 끝낸 것만 쓴다.
// 결과가 없으면 fetch하지 않은 것으로 보고 upstream_fetch로 거부한다.
func validateIssueOpsPhaseTransition(stateRoot string, record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase, upstream issueOpsUpstreamFetcher) error {
	if record.Phase == IssueOpsPhaseDone {
		return fmt.Errorf("cannot leave done phase")
	}
	if issueOpsPhaseRank(phase) < issueOpsPhaseRank(record.Phase) {
		return fmt.Errorf("cannot move issueops phase backward from %s to %s", record.Phase, phase)
	}
	// Fail-closed (rules 1/8): problem and grill have no other readiness gate, so
	// these are the only enforcement of the problem/grill completion contracts.
	// grill entry requires problem complete; plan entry requires grill complete.
	// Downstream phases keep their own readiness gates (which transitively require
	// plan, and thus grill, on the normal sequential path).
	if phase == IssueOpsPhaseGrill {
		if ready := IssueOpsProblemReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter grill phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhasePlan {
		// Plan readiness first: it carries intent_contract/issue_url/plan_prep, so
		// the most fundamental missing key surfaces before the grill-completion
		// delta (split_decision/domain_review/branch).
		if ready := IssueOpsPlanReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter plan phase: missing %s", strings.Join(ready.Missing, ", "))
		}
		if ready := IssueOpsGrillReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter plan phase: grill incomplete: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseCompatibilityReview {
		if ready := IssueOpsCompatibilityReviewReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter compatibility-review phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseImplement {
		if ready := IssueOpsImplementationReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter implement phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseAISlopClean {
		if ready := IssueOpsAISlopCleanReadiness(record); !ready.Ready {
			return fmt.Errorf("cannot enter ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseFeedback && strings.TrimSpace(record.AISlopCleanAt) == "" {
		return fmt.Errorf("cannot enter feedback phase before ai-slop-clean phase")
	}
	if phase == IssueOpsPhasePR {
		if upstream == nil {
			upstream = func(gitRoot string) issueOpsUpstreamFetch {
				return issueOpsUpstreamFetch{gitRoot: gitRoot, failed: true, stderr: "upstream was not fetched before the pr transition; retry the command"}
			}
		}
		if ready := issueOpsStrictPRReadinessWithStateUsing(stateRoot, record, upstream); !ready.Ready {
			return fmt.Errorf("cannot enter pr phase: missing %s", strings.Join(ready.Missing, ", "))
		}
	}
	if phase == IssueOpsPhaseDone && record.Phase != IssueOpsPhasePR {
		return fmt.Errorf("cannot enter done phase before pr phase")
	}
	if phase == IssueOpsPhaseDone {
		if missing := issueOpsRemoteArtifactMissing(record); len(missing) > 0 {
			return fmt.Errorf("cannot enter done phase before remote artifact verification: missing %s", strings.Join(missing, ", "))
		}
		if record.Execution == nil || record.Execution.Completion == nil || record.Execution.Lease.Status != issueopscontract.LeaseStatusReleased {
			return fmt.Errorf("cannot enter done phase before issueops execution completion")
		}
	}
	return nil
}

func applyIssueOpsPhaseTransition(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase) issueops.IssueOpsRecord {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return applyIssueOpsPhaseTransitionAt(record, phase, now)
}

func applyIssueOpsPhaseTransitionAt(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase, now string) issueops.IssueOpsRecord {
	prevPhase := record.Phase
	record.Phase = phase
	if phase == IssueOpsPhaseAISlopClean && strings.TrimSpace(record.AISlopCleanAt) == "" {
		record.AISlopCleanAt = now
	}
	if phase == IssueOpsPhaseAISlopClean {
		record.AISlopCleanHead = issueOpsCurrentHead(record)
		record.AISlopCleanFingerprint = implementation.ChangeFingerprint(record)
	}
	record.PhaseLedger = stampIssueOpsForwardTransition(record.PhaseLedger, prevPhase, phase, now)
	return record
}
