// Package status holds the contract DTOs of the harness status report.
package status

import (
	doctorcontract "issueops/internal/contract/doctor"
	inspect "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
)

// Result is the harness status report: the inspect, doctor, state and worker
// observations plus the latest self-verify status. OK is false when any
// observation failed, and Warnings explains why.
type Result struct {
	OK         bool                               `json:"ok"`
	Kind       string                             `json:"kind"`
	Version    string                             `json:"version"`
	Repo       string                             `json:"repo"`
	Inspect    inspect.Info                       `json:"inspect"`
	Doctor     doctorcontract.HarnessDoctorResult `json:"doctor"`
	State      statecontract.ListResult           `json:"state"`
	Workers    workercontract.ListResult          `json:"workers"`
	SelfVerify SelfVerifyStatus                   `json:"self_verify"`
	Warnings   []string                           `json:"warnings"`
}

// SelfVerifyStatus describes the newest saved self-verify record: its state key,
// last update time and size. Found is false when no record exists.
type SelfVerifyStatus struct {
	LatestKey string `json:"latest_key,omitempty"`
	Found     bool   `json:"found"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
}
