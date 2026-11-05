// Package status holds the domain rule that turns the facts gathered for
// `issueops status` (doctor, state, workers, latest self-verify record) into
// one overall decision with warnings. It performs no I/O.
package status

import (
	statuscontract "issueops/internal/contract/status"
)

// Outcome is the result of one status probe: whether it passed and the error that stopped it, if any.
type Outcome struct {
	OK  bool
	Err error
}

// Record identifies a stored checkpoint by key, update time, and size in bytes.
type Record struct {
	Key, UpdatedAt string
	Bytes          int
}

// Observation is everything Evaluate needs: the doctor, state, and workers
// outcomes, the latest self-verify Record if one was found, and the warnings
// and read failure flag from looking it up.
type Observation struct {
	Doctor, State, Workers Outcome
	LatestSelfVerify       *Record
	SelfVerifyWarnings     []string
	SelfVerifyReadFailed   bool
}

// Decision is the overall status verdict with the warnings behind it and the
// self-verify summary to report.
type Decision struct {
	OK         bool
	Warnings   []string
	SelfVerify statuscontract.SelfVerifyStatus
}

// Evaluate combines facts into a Decision. OK requires doctor, state, and
// workers to pass with no error and the self-verify lookup to have succeeded.
// Probe errors and self-verify warnings are prefixed with their source and
// returned as warnings.
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
