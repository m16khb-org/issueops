package issueopscycle

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	cycleport "issueops/internal/port/issueopscycle"
)

func RefreshAISlopClean(store cycleport.AISlopCleanRefreshStore, stateRoot string, record model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if ready := store.Readiness(record); !ready.Ready {
		return model.IssueOpsRecord{OK: false}, fmt.Errorf("cannot refresh ai-slop-clean phase: missing %s", strings.Join(ready.Missing, ", "))
	}
	record.AISlopCleanAt = store.Now()
	record.AISlopCleanHead = store.Head(record)
	record.AISlopCleanFingerprint = store.Fingerprint(record)
	return store.TouchWrite(stateRoot, record)
}
