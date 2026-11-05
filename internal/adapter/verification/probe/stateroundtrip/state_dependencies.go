package stateroundtrip

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
)

import statecontract "issueops/internal/contract/state"

// Validator runs the state roundtrip check with injected state read and write
// effects.
type Validator struct {
	StateRead     func(string, string) (statecontract.Result, error)
	WriteRecord   func(string, string, statecontract.RecordEnvelope) (string, error)
	WriteSnapshot func(string, string, selfaugment.StateSnapshot) error
	OpenDatabase  func(string) (StateDatabase, error)
}

// Validate writes a record and a self-verification snapshot into a throwaway
// state directory and reads them back through the issueops binary at binary,
// reporting a failed step if anything differs. seed makes the record key and
// content deterministic.
func (v Validator) Validate(binary, root string, seed int64) selfverify.StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, stateRoundtripValidationDeps{stateRead: v.StateRead, writeRecord: v.WriteRecord, writeSnapshot: v.WriteSnapshot, openDatabase: v.OpenDatabase})
}
