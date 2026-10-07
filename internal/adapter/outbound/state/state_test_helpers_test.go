package state

import (
	"issueops/internal/adapter/outbound/sqlstore"
	statecontract "issueops/internal/contract/state"
)

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func stateDoctorHasIssue(issues []statecontract.StateDoctorIssue, code string) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

// writeRawStateRow inserts raw record bytes directly into the state store,
// bypassing NewService().Write normalization, to simulate corrupt or foreign rows.
func writeRawStateRow(t interface {
	Helper()
	Fatal(args ...any)
}, dir, key, raw string) {
	t.Helper()
	db, err := sqlstore.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Put(stateBucket, key, []byte(raw)); err != nil {
		t.Fatal(err)
	}
}
