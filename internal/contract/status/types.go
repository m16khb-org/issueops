package status

import (
	doctorcontract "issueops/internal/contract/doctor"
	inspect "issueops/internal/contract/inspect"
	statecontract "issueops/internal/contract/state"
	workercontract "issueops/internal/contract/worker"
)

type Result struct {
	OK         bool                               `json:"ok"`
	Kind       string                             `json:"kind"`
	Version    string                             `json:"version"`
	Repo       string                             `json:"repo"`
	Inspect    inspect.InspectInfo                `json:"inspect"`
	Doctor     doctorcontract.HarnessDoctorResult `json:"doctor"`
	State      statecontract.StateListResult      `json:"state"`
	Workers    workercontract.WorkerListResult    `json:"workers"`
	SelfVerify SelfVerifyStatus                   `json:"self_verify"`
	Warnings   []string                           `json:"warnings"`
}

type SelfVerifyStatus struct {
	LatestKey string `json:"latest_key,omitempty"`
	Found     bool   `json:"found"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
}
