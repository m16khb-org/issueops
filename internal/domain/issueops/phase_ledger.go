package issueops

import (
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

// StampForwardTransition records the observed completion and entry timestamps.
func StampForwardTransition(ledger issueopscontract.PhaseLedger, previousPhase, newPhase issueopscontract.Phase, now string, artifacts []string) issueopscontract.PhaseLedger {
	if ledger == nil {
		ledger = issueopscontract.PhaseLedger{}
	}
	previous := ledger[previousPhase]
	previous.Phase = previousPhase
	if previous.EnteredAt == "" {
		previous.EnteredAt = now
	}
	previous.CompletedAt = now
	previous.Artifacts = artifacts
	previous.Missing = nil
	previous.Notes = clearStaleLedgerNotes(previous.Notes)
	ledger[previousPhase] = previous

	entry := ledger[newPhase]
	entry.Phase = newPhase
	if entry.EnteredAt == "" {
		entry.EnteredAt = now
	}
	ledger[newPhase] = entry
	return ledger
}

func clearStaleLedgerNotes(notes []string) []string {
	if len(notes) == 0 {
		return notes
	}
	kept := make([]string, 0, len(notes))
	for _, note := range notes {
		if strings.HasPrefix(note, "stale:") {
			continue
		}
		kept = append(kept, note)
	}
	if len(kept) == 0 {
		return nil
	}
	return kept
}

// MarkLedgerStale preserves the audit entry while invalidating its completion.
func MarkLedgerStale(ledger issueopscontract.PhaseLedger, note string, phases ...issueopscontract.Phase) issueopscontract.PhaseLedger {
	if ledger == nil {
		ledger = issueopscontract.PhaseLedger{}
	}
	for _, phase := range phases {
		entry := ledger[phase]
		entry.Phase = phase
		entry.CompletedAt = ""
		entry.Notes = append(entry.Notes, note)
		ledger[phase] = entry
	}
	return ledger
}
