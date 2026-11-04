// Package issueopsstatus is the domain projection that fills a cycle record's
// phase ledger: for every phase the stored ledger lacks, it derives an entry
// from the record's readiness. It has no side effects of its own; readiness
// observation is injected as a Completion function.
package issueopsstatus

import (
	"maps"

	issueopsstatuscontract "issueops/internal/contract/issueopsstatus"
)

// Completion reports whether phase is complete for record and, when it is not,
// what is missing. Implementations may observe external state such as git.
type Completion func(
	issueopsstatuscontract.Record,
	issueopsstatuscontract.Phase,
) issueopsstatuscontract.Readiness

// Projector derives phase ledger entries for a record using its Completion
// function.
type Projector struct {
	completion Completion
}

// NewProjector returns a Projector that asks completion for each phase's readiness.
func NewProjector(completion Completion) Projector {
	return Projector{completion: completion}
}

// Project fills each phase the stored ledger lacks with a derived entry. A
// recorded phase is never re-derived, so its completion (which may observe
// git) is not computed. Completion always sees the stored ledger.
func (projector Projector) Project(record issueopsstatuscontract.Record) issueopsstatuscontract.Record {
	ledger := make(issueopsstatuscontract.PhaseLedger, len(phases))
	maps.Copy(ledger, record.PhaseLedger)
	for _, phase := range phases {
		if _, exists := ledger[phase]; !exists {
			ledger[phase] = projector.deriveEntry(record, phase)
		}
	}
	record.PhaseLedger = ledger
	return record
}

// Derive returns a ledger with a derived entry for every phase, ignoring any
// ledger already stored on record. Project, in contrast, keeps recorded phases.
func (projector Projector) Derive(record issueopsstatuscontract.Record) issueopsstatuscontract.PhaseLedger {
	ledger := issueopsstatuscontract.PhaseLedger{}
	for _, phase := range phases {
		ledger[phase] = projector.deriveEntry(record, phase)
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

func (projector Projector) deriveEntry(record issueopsstatuscontract.Record, phase issueopsstatuscontract.Phase) issueopsstatuscontract.PhaseLedgerEntry {
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

// ArtifactKeys returns the artifact names a completed phase is expected to have
// produced, or nil for a phase it does not know.
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
