package issueopsstatus

import issueopsstatuscontract "issueops/internal/contract/issueopsstatus"

type Completion func(
	issueopsstatuscontract.Record,
	issueopsstatuscontract.Phase,
) issueopsstatuscontract.Readiness

type Projector struct {
	completion Completion
}

func NewProjector(completion Completion) Projector {
	return Projector{completion: completion}
}

// Project fills each phase the stored ledger lacks with a derived entry. A
// recorded phase is never re-derived, so its completion (which may observe
// git) is not computed. Completion always sees the stored ledger.
func (projector Projector) Project(record issueopsstatuscontract.Record) issueopsstatuscontract.Record {
	ledger := make(issueopsstatuscontract.PhaseLedger, len(phases))
	for phase, entry := range record.PhaseLedger {
		ledger[phase] = entry
	}
	for _, phase := range phases {
		if _, exists := ledger[phase]; !exists {
			ledger[phase] = projector.derive(record, phase)
		}
	}
	record.PhaseLedger = ledger
	return record
}

func (projector Projector) Derive(record issueopsstatuscontract.Record) issueopsstatuscontract.PhaseLedger {
	ledger := issueopsstatuscontract.PhaseLedger{}
	for _, phase := range phases {
		ledger[phase] = projector.derive(record, phase)
	}
	return ledger
}

var phases = [...]issueopsstatuscontract.Phase{
	issueopsstatuscontract.PhaseProblem,
	issueopsstatuscontract.PhaseGrill,
	issueopsstatuscontract.PhasePlan,
	issueopsstatuscontract.PhaseCompatibilityReview,
	issueopsstatuscontract.PhaseImplement,
	issueopsstatuscontract.PhaseAISlopClean,
	issueopsstatuscontract.PhaseFeedback,
	issueopsstatuscontract.PhasePR,
	issueopsstatuscontract.PhaseDone,
}

func (projector Projector) derive(record issueopsstatuscontract.Record, phase issueopsstatuscontract.Phase) issueopsstatuscontract.PhaseLedgerEntry {
	entry := issueopsstatuscontract.PhaseLedgerEntry{
		Phase: phase,
		Notes: []string{"derived"},
	}
	completion := projector.completion(record, phase)
	if completion.Ready {
		entry.CompletedAt = ""
		entry.Artifacts = ArtifactKeys(phase)
	} else {
		entry.Missing = completion.Missing
	}
	return entry
}

func ArtifactKeys(phase issueopsstatuscontract.Phase) []string {
	switch phase {
	case issueopsstatuscontract.PhaseProblem:
		return []string{"intent_contract"}
	case issueopsstatuscontract.PhaseGrill:
		return []string{"issue_url", "branch", "plan_prep", "split_decision", "domain_review"}
	case issueopsstatuscontract.PhasePlan:
		return []string{"branch_prepare", "worktree_path", "plan_path", "architecture_review"}
	case issueopsstatuscontract.PhaseCompatibilityReview:
		return []string{"compatibility_review", "compatibility_approval", "compatibility_blockers"}
	case issueopsstatuscontract.PhaseImplement:
		return []string{"worktree_tools", "execution_decision", "implementation_changes"}
	case issueopsstatuscontract.PhaseAISlopClean:
		return []string{"ai_slop_clean_at", "ai_slop_clean_head", "ai_slop_clean_fingerprint", "cleanup_evidence", "verification_evidence"}
	case issueopsstatuscontract.PhaseFeedback:
		return []string{"feedback_classification", "contract_feedback_issue_update", "feedback_resolution"}
	case issueopsstatuscontract.PhasePR:
		return []string{"strict_pr_readiness", "children_complete", "remote_artifact", "target_branch_match"}
	case issueopsstatuscontract.PhaseDone:
		return []string{"prior_phase_pr", "verified_remote_artifact"}
	default:
		return nil
	}
}
