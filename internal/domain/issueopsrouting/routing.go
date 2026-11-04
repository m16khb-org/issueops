// Package issueopsrouting scores whether a cycle followed the expected skill
// routing and maintains its bounded routing trace. It is a pure domain package.
package issueopsrouting

import (
	"fmt"
	"strings"
	"time"

	issueopsroutingcontract "issueops/internal/contract/issueopsrouting"
)

const maxTraceEntries = 500

// NewEntry builds a routing trace entry stamped with at in UTC. phase and
// skill are trimmed, must be non-empty, and may be at most 64 bytes each.
func NewEntry(phase, skill string, at time.Time) (issueopsroutingcontract.Entry, error) {
	phase = strings.TrimSpace(phase)
	skill = strings.TrimSpace(skill)
	if phase == "" {
		return issueopsroutingcontract.Entry{}, fmt.Errorf("routing phase is required (--phase)")
	}
	if skill == "" {
		return issueopsroutingcontract.Entry{}, fmt.Errorf("routing skill is required (--skill)")
	}
	if len(phase) > 64 || len(skill) > 64 {
		return issueopsroutingcontract.Entry{}, fmt.Errorf(
			"routing phase and skill must not exceed 64 bytes each",
		)
	}
	return issueopsroutingcontract.Entry{
		Phase: phase,
		Skill: skill,
		At:    at.UTC().Format(time.RFC3339Nano),
	}, nil
}

// Append adds entry to the record's routing trace and stamps UpdatedAt. A
// phase and skill pair already in the trace (case-insensitive) is a no-op that
// returns changed false. A full trace (500 entries) is an error.
func Append(
	record issueopsroutingcontract.Record,
	entry issueopsroutingcontract.Entry,
) (issueopsroutingcontract.Record, bool, error) {
	for _, observed := range record.RoutingTrace {
		if strings.EqualFold(observed.Phase, entry.Phase) &&
			strings.EqualFold(observed.Skill, entry.Skill) {
			return record, false, nil
		}
	}
	if len(record.RoutingTrace) >= maxTraceEntries {
		return issueopsroutingcontract.Record{OK: false}, false, fmt.Errorf(
			"routing trace is full (%d entries)",
			maxTraceEntries,
		)
	}
	record.RoutingTrace = append(record.RoutingTrace, entry)
	record.UpdatedAt = entry.At
	record.OK = true
	return record, true, nil
}

// ScoreRecord checks the record's routing trace against expected pairs and
// reports the pairs it lacks. Phase and skill compare case-insensitively.
func ScoreRecord(
	record issueopsroutingcontract.Record,
	expected []issueopsroutingcontract.Expected,
) issueopsroutingcontract.Result {
	missing := make([]issueopsroutingcontract.Expected, 0)
	for _, expectedEntry := range expected {
		if !containsRecordPairing(record.RoutingTrace, expectedEntry) {
			missing = append(missing, expectedEntry)
		}
	}
	return issueopsroutingcontract.Result{OK: len(missing) == 0, Missing: missing}
}

// Score checks observed phase and skill pairs against expected pairs and
// reports the missing ones. OK is true when nothing is missing.
func Score(
	expected []issueopsroutingcontract.Expected,
	observed []issueopsroutingcontract.Expected,
) issueopsroutingcontract.Result {
	missing := make([]issueopsroutingcontract.Expected, 0)
	for _, expectedEntry := range expected {
		if !containsPairing(observed, expectedEntry) {
			missing = append(missing, expectedEntry)
		}
	}
	return issueopsroutingcontract.Result{OK: len(missing) == 0, Missing: missing}
}

func containsPairing(
	trace []issueopsroutingcontract.Expected,
	expected issueopsroutingcontract.Expected,
) bool {
	for _, entry := range trace {
		if strings.EqualFold(strings.TrimSpace(entry.Phase), strings.TrimSpace(expected.Phase)) &&
			strings.EqualFold(strings.TrimSpace(entry.Skill), strings.TrimSpace(expected.Skill)) {
			return true
		}
	}
	return false
}

func containsRecordPairing(
	trace []issueopsroutingcontract.Entry,
	expected issueopsroutingcontract.Expected,
) bool {
	for _, entry := range trace {
		if strings.EqualFold(strings.TrimSpace(entry.Phase), strings.TrimSpace(expected.Phase)) &&
			strings.EqualFold(strings.TrimSpace(entry.Skill), strings.TrimSpace(expected.Skill)) {
			return true
		}
	}
	return false
}
