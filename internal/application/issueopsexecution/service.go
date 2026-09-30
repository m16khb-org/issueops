package issueopsexecution

import (
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

// Service binds execution dispatch and recovery to one set of observations.
type Service struct {
	ReadRecord     func(string, string) (model.IssueOpsRecord, error)
	SamePath       func(string, string) bool
	InspectProcess cycleapp.NativeProcessInspector
}
