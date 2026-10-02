package issueopsexecution

import (
	model "issueops/internal/contract/issueops"
	authorityport "issueops/internal/port/authority"
)

// Service binds execution dispatch and recovery to one set of observations.
type Service struct {
	ReadRecord func(string, string) (model.IssueOpsRecord, error)
	SamePath   func(string, string) bool
	Verifier   authorityport.ActorVerifier
}
