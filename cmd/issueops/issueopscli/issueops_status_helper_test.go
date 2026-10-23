package issueopscli

import (
	"encoding/json"
	"testing"
)

// statusRecordAfter runs a record mutation with --json, checks it answered
// with the compact acknowledgement for id, and returns the full record read
// back through `status`, which is where callers find the mutation's effect.
func statusRecordAfter(t *testing.T, id string, mutation []string) map[string]any {
	t.Helper()
	out := captureStdoutForContract(t, func() error { return runIssueOps(mutation) })
	var ack map[string]any
	if err := json.Unmarshal([]byte(out), &ack); err != nil {
		t.Fatalf("%v should answer one JSON ack: %v\n%s", mutation[:2], err, out)
	}
	if ack["ok"] != true || ack["id"] != id {
		t.Fatalf("%v ack = %#v", mutation[:2], ack)
	}
	return statusRecordForTest(t, id)
}

// statusRecordForTest reads the full record through `status --json`.
func statusRecordForTest(t *testing.T, id string) map[string]any {
	t.Helper()
	status := captureStdoutForContract(t, func() error {
		return runIssueOps([]string{"status", "--id", id, "--json"})
	})
	var record map[string]any
	if err := json.Unmarshal([]byte(status), &record); err != nil {
		t.Fatalf("status JSON: %v\n%s", err, status)
	}
	return record
}

func statusJSONAfter(t *testing.T, id string, mutation []string) string {
	t.Helper()
	b, err := json.Marshal(statusRecordAfter(t, id, mutation))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
