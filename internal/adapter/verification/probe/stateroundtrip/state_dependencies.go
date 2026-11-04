package stateroundtrip

import (
	selfaugment "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
)

import statecontract "issueops/internal/contract/state"

type Validator struct {
	StateRead     func(string, string) (statecontract.Result, error)
	WriteRecord   func(string, string, statecontract.RecordEnvelope) (string, error)
	WriteSnapshot func(string, string, selfaugment.StateSnapshot) error
	OpenDatabase  func(string) (StateDatabase, error)
}

func (v Validator) Validate(binary, root string, seed int64) selfverify.StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, stateRoundtripValidationDeps{stateRead: v.StateRead, writeRecord: v.WriteRecord, writeSnapshot: v.WriteSnapshot, openDatabase: v.OpenDatabase})
}
