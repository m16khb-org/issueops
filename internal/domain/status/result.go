package status

import (
	statuscontract "issueops/internal/contract/status"
)

type Outcome struct {
	OK  bool
	Err error
}
type Record struct {
	Key, UpdatedAt string
	Bytes          int
}
type Observation struct {
	Doctor, State, Workers Outcome
	LatestSelfVerify       *Record
	SelfVerifyWarnings     []string
	SelfVerifyReadFailed   bool
}
type Decision struct {
	OK         bool
	Warnings   []string
	SelfVerify statuscontract.SelfVerifyStatus
}

func Evaluate(facts Observation) Decision {
	warnings := []string{}
	if facts.Doctor.Err != nil {
		warnings = append(warnings, "doctor: "+facts.Doctor.Err.Error())
	}
	if facts.State.Err != nil {
		warnings = append(warnings, "state: "+facts.State.Err.Error())
	}
	if facts.Workers.Err != nil {
		warnings = append(warnings, "workers: "+facts.Workers.Err.Error())
	}
	selfVerify := statuscontract.SelfVerifyStatus{}
	lookupOK := len(warnings) == 0 && !facts.SelfVerifyReadFailed
	for _, warning := range facts.SelfVerifyWarnings {
		warnings = append(warnings, "selfverify: "+warning)
	}
	if record := facts.LatestSelfVerify; record != nil {
		selfVerify = statuscontract.SelfVerifyStatus{LatestKey: record.Key, Found: true, UpdatedAt: record.UpdatedAt, Bytes: record.Bytes}
	}
	return Decision{OK: lookupOK && facts.Doctor.OK && facts.State.OK && facts.Workers.OK, Warnings: warnings, SelfVerify: selfVerify}
}
