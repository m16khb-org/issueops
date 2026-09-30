package selfaugment

import (
	"reflect"
	"testing"
)

func TestCandidateExportKeepsBuiltinCurriculumRegardlessOfSourcePresence(t *testing.T) {
	present, missing := NewCandidateExport(true), NewCandidateExport(false)
	if !present.OK || present.Kind != "self_verification_candidate_export" || present.LoopKind != "self_verification" || present.KoreanName != "자기 검증 루프" {
		t.Fatalf("identity drift: %+v", present)
	}
	if !reflect.DeepEqual(present.Candidates, missing.Candidates) || present.CandidateCount != missing.CandidateCount || len(present.Candidates) != present.CandidateCount {
		t.Fatal("missing source changed builtin curriculum")
	}
	if present.OpenCandidateIDs == nil || len(present.OpenCandidateIDs) != 0 || present.SelectedCandidate != nil {
		t.Fatalf("completed curriculum became open: %+v", present)
	}
	if len(present.SatisfiedCandidateIDs) != present.CandidateCount {
		t.Fatalf("candidate projection dropped IDs: %+v", present)
	}
	for i, candidate := range present.Candidates {
		if candidate.ID != present.SatisfiedCandidateIDs[i] {
			t.Fatalf("catalog order changed at %d", i)
		}
	}
	if len(present.Warnings) != 0 || len(missing.Warnings) != 1 || !present.SourceExists || missing.SourceExists {
		t.Fatal("source observation policy changed")
	}
}
