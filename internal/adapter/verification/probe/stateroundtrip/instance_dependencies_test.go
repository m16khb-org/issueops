package stateroundtrip

import (
	"errors"
	"strings"
	"testing"

	statecontract "issueops/internal/contract/state"
)

func TestStateRoundtripKeepsPreparedRecordWriter(t *testing.T) {
	first := func(string, string, statecontract.RecordEnvelope) (string, error) {
		return "", errors.New("first record writer refused")
	}
	second := func(string, string, statecontract.RecordEnvelope) (string, error) {
		return "", errors.New("second record writer refused")
	}
	deps := testStateDependencies(stateRoundtripTestDeps(t, 456))
	deps.writeRecord = first
	secondDeps := deps
	secondDeps.writeRecord = second
	for _, tc := range []struct {
		deps stateRoundtripValidationDeps
		want string
	}{{deps, "first record writer refused"}, {secondDeps, "second record writer refused"}, {deps, "first record writer refused"}} {
		step := validateStateRoundtripWithDeps("bin", t.TempDir(), 456, tc.deps)
		if step.OK || !strings.Contains(step.Error, tc.want) {
			t.Fatalf("prepared writer lost: %+v", step)
		}
	}
}
