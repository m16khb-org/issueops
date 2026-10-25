package trace

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	statecontract "issueops/internal/contract/state"
)

// Probe: a missing trace file path fell through to the state lookup and
// answered "invalid state key"; a path is reported as the missing file.
func TestLoadReportsAMissingPathAsAFile(t *testing.T) {
	source := Source{ReadState: func(string) (statecontract.StateResult, error) {
		t.Fatal("a path must not be read as a state key")
		return statecontract.StateResult{}, nil
	}}
	_, _, _, err := source.Load(filepath.Join(t.TempDir(), "nope.json"), "issueops")
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("err = %v", err)
	}
}
