package state

import (
	statecontract "issueops/internal/contract/state"
	"testing"
)

func TestDoctorRecognizesOnlyTheCanonicalRecordWriteLeaseFile(t *testing.T) {
	entries := []DoctorEntry{
		{Name: statecontract.RecordWriteLeaseFile, Path: "state/" + statecontract.RecordWriteLeaseFile},
		{Name: "another.lock", Path: "state/another.lock"},
		{Name: statecontract.RecordWriteLeaseFile + ".bak", Path: "state/" + statecontract.RecordWriteLeaseFile + ".bak"},
	}
	result := InspectDoctor("state", entries, nil)
	if len(result.Issues) != 2 {
		t.Fatalf("expected only the two foreign files, got %+v", result.Issues)
	}
	for _, issue := range result.Issues {
		if issue.Code != "unexpected_file" || issue.Path == entries[0].Path {
			t.Fatalf("canonical lease file misclassified: %+v", issue)
		}
	}
}
