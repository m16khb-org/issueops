package stateroundtrip

import statecontract "issueops/internal/contract/state"

type Validator struct {
	StateRead     func(string, string) (statecontract.StateResult, error)
	WriteRecord   func(string, string, statecontract.RecordEnvelope) (string, error)
	WriteSnapshot func(string, string, SelfAugmentStateSnapshot) error
	OpenDatabase  func(string) (StateDatabase, error)
}

func (v Validator) Validate(binary, root string, seed int64) StepResult {
	return validateStateRoundtripWithDeps(binary, root, seed, stateRoundtripValidationDeps{stateRead: v.StateRead, writeRecord: v.WriteRecord, writeSnapshot: v.WriteSnapshot, openDatabase: v.OpenDatabase})
}
