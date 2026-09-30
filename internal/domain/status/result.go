package status

import (
	statuscontract "issueops/internal/contract/status"
	"strings"
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
	Records                []Record
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
	for _, record := range facts.Records {
		if record.Key == "self-verify-latest" || strings.HasPrefix(record.Key, "self-verify") {
			selfVerify = statuscontract.SelfVerifyStatus{LatestKey: record.Key, Found: true, UpdatedAt: record.UpdatedAt, Bytes: record.Bytes}
			break
		}
	}
	return Decision{OK: len(warnings) == 0 && facts.Doctor.OK && facts.State.OK && facts.Workers.OK, Warnings: warnings, SelfVerify: selfVerify}
}

func DaemonAdmissionObserved(running, reachable, identityVerified bool) bool {
	return running && reachable && identityVerified
}
