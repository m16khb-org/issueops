package issueops

import (
	"time"

	"issueops/internal/adapter/issueops/implementation"
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func refreshIssueOpsAISlopClean(stateRoot string, record issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
	return cycleapp.RefreshAISlopClean(cycleport.AISlopCleanRefreshStore{
		Readiness:   IssueOpsAISlopCleanReadiness,
		Now:         func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
		Head:        issueOpsCurrentHead,
		Fingerprint: implementation.ChangeFingerprint,
		TouchWrite:  touchAndWriteIssueOps,
	}, stateRoot, record)
}
