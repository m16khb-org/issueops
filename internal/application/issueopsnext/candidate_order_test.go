package issueopsnext

import (
	"context"
	"fmt"
	"slices"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsinventorydomain "issueops/internal/domain/issueopsinventory"
)

// candidateIDs runs Next over entries and returns the candidate ids in order.
func candidateIDs(t *testing.T, entries []issueopsinventorycontract.ListEntry) []string {
	t.Helper()
	result, err := NewService(testPorts(entries, nil)).Next(context.Background(), "/state", "/repo", "")
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.ID)
	}
	return ids
}

// planEntry builds an active plan-phase entry with the given record times.
func planEntry(id, lastRecordAt, updatedAt string) issueopsinventorycontract.ListEntry {
	return issueopsinventorycontract.ListEntry{
		ID: id, Phase: issueopscontract.PhasePlan, LastRecordAt: lastRecordAt, UpdatedAt: updatedAt,
	}
}

// A lease claim does not touch Record.UpdatedAt, so the cut must follow the
// projected LastRecordAt: the freshly claimed cycle stays and leads.
func TestNextKeepsTheMostRecentlyActiveCandidates(t *testing.T) {
	records := []issueopscontract.Record{
		{
			ID: "io-claimed", Phase: issueopscontract.PhasePlan, UpdatedAt: "2026-08-01T00:00:00Z",
			Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{ClaimedAt: "2026-10-30T00:00:00Z"}},
		},
		{
			ID: "io-oldest", Phase: issueopscontract.PhasePlan, UpdatedAt: "2026-09-01T00:00:00Z",
			Execution: &issueopscontract.Execution{Lease: issueopscontract.WriteLease{ClaimedAt: "2026-09-01T00:00:00Z"}},
		},
	}
	for i := range 19 {
		records = append(records, issueopscontract.Record{
			ID: fmt.Sprintf("io-%02d", i), Phase: issueopscontract.PhasePlan,
			UpdatedAt: fmt.Sprintf("2026-10-25T00:00:%02dZ", i),
		})
	}
	entries := make([]issueopsinventorycontract.ListEntry, 0, len(records))
	for _, record := range records {
		entries = append(entries, issueopsinventorydomain.ProjectEntry(record))
	}
	ids := candidateIDs(t, entries)
	if len(ids) != 20 || ids[0] != "io-claimed" || slices.Contains(ids, "io-oldest") {
		t.Fatalf("candidates = %q", ids)
	}
}

// Text order of RFC3339Nano values is not time order: trailing zeros are
// trimmed and fractions differ in length. The four newest are checked in
// instant order, while UpdatedAt text order is the reverse of it.
func TestNextOrdersCandidatesByInstantNotText(t *testing.T) {
	instants := []struct{ id, at string }{
		{"io-t0", "2026-10-25T14:24:40Z"},
		{"io-t1", "2026-10-25T14:24:40.37Z"},
		{"io-t2", "2026-10-25T14:24:40.3762739Z"},
		{"io-t3", "2026-10-25T14:24:40.5Z"},
	}
	var entries []issueopsinventorycontract.ListEntry
	for i, instant := range instants {
		entries = append(entries, planEntry(instant.id, instant.at, fmt.Sprintf("2026-11-01T00:00:0%dZ", len(instants)-i)))
	}
	for i := range 22 {
		stamp := fmt.Sprintf("2026-01-01T00:00:%02dZ", i)
		entries = append(entries, planEntry(fmt.Sprintf("io-old-%02d", i), stamp, stamp))
	}
	ids := candidateIDs(t, entries)
	want := []string{"io-t3", "io-t2", "io-t1", "io-t0"}
	if len(ids) != 20 || !slices.Equal(ids[:4], want) {
		t.Fatalf("candidates = %q, want leading %q", ids, want)
	}
}

// An entry without LastRecordAt falls back to UpdatedAt, compared as an
// instant: 23:00+09:00 is 14:00Z and so precedes 15:00Z.
func TestNextFallsBackToUpdatedAtWhenRecordTimeIsMissing(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{
		planEntry("io-kst", "", "2026-10-25T23:00:00+09:00"),
		planEntry("io-utc", "2026-10-25T15:00:00Z", "2026-10-25T15:00:00Z"),
	}
	for i := range 20 {
		stamp := fmt.Sprintf("2026-01-01T00:00:%02dZ", i)
		entries = append(entries, planEntry(fmt.Sprintf("io-old-%02d", i), "", stamp))
	}
	ids := candidateIDs(t, entries)
	if len(ids) != 20 || ids[0] != "io-utc" || ids[1] != "io-kst" {
		t.Fatalf("candidates = %q", ids)
	}
}

// A value that does not parse is the oldest, even though its text sorts after
// every digit-leading timestamp.
func TestNextTreatsUnparseableTimesAsOldest(t *testing.T) {
	entries := []issueopsinventorycontract.ListEntry{planEntry("io-bad", "not-a-time", "not-a-time")}
	for i := range 20 {
		stamp := fmt.Sprintf("2026-10-25T00:00:%02dZ", i)
		entries = append(entries, planEntry(fmt.Sprintf("io-%02d", i), stamp, stamp))
	}
	ids := candidateIDs(t, entries)
	if len(ids) != 20 || slices.Contains(ids, "io-bad") {
		t.Fatalf("candidates = %q", ids)
	}
}

// At or below the bound nothing is cut, so the candidates keep input order
// even when the times are mixed.
func TestNextKeepsInputOrderAtTheCandidateBound(t *testing.T) {
	var entries []issueopsinventorycontract.ListEntry
	var want []string
	for i := range maxCandidates {
		id := fmt.Sprintf("io-%02d", i)
		stamp := fmt.Sprintf("2026-10-25T00:00:%02dZ", (i*7)%maxCandidates)
		entries = append(entries, planEntry(id, stamp, stamp))
		want = append(want, id)
	}
	if ids := candidateIDs(t, entries); !slices.Equal(ids, want) {
		t.Fatalf("candidates = %q, want %q", ids, want)
	}
}

// Candidates that share an instant keep their input order: three blocks of
// tied entries are interleaved in the input, and the cut keeps the newest block
// whole, then the middle block in input order up to the bound.
func TestNextKeepsInputOrderForCandidatesWithTheSameInstant(t *testing.T) {
	stamps := []string{"2026-10-25T00:00:00Z", "2026-10-26T00:00:00Z", "2026-10-27T00:00:00Z"}
	var entries []issueopsinventorycontract.ListEntry
	var newest, middle []string
	for i := range 45 {
		id := fmt.Sprintf("io-%02d", i)
		stamp := stamps[i%len(stamps)]
		entries = append(entries, planEntry(id, stamp, stamp))
		switch i % len(stamps) {
		case 2:
			newest = append(newest, id)
		case 1:
			middle = append(middle, id)
		}
	}
	want := append(slices.Clone(newest), middle[:maxCandidates-len(newest)]...)
	if ids := candidateIDs(t, entries); !slices.Equal(ids, want) {
		t.Fatalf("candidates = %q, want %q", ids, want)
	}
}
