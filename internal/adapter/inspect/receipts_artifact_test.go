package inspect

import (
	"path/filepath"
	"testing"

	inspectcontract "issueops/internal/contract/inspect"
)

// 실제 host 실행 artifact(Claude stream-json, Codex app-server·step 요약, Omo transport run)에서
// 줄여 만든 fixture다. 성공 호출만 verified로 올라가는지, 실패·언급만 있는 artifact는 내려가는지 본다.
func TestHostsReceiptArtifactEventSemantics(t *testing.T) {
	type want struct{ status, reason string }
	verified := want{"verified", ""}
	cases := []struct {
		fixture                         string
		discovered, connected, protocol want
	}{
		{"claude-stream.jsonl", verified, verified, want{"unknown", "receipt_artifact_missing_revision"}},
		{"claude-stream-error.jsonl", verified, want{"unknown", "receipt_artifact_tool_failed"}, want{"unknown", "receipt_artifact_missing_revision"}},
		{"codex-app-server.jsonl", verified, verified, want{"unknown", "receipt_artifact_missing_revision"}},
		{"codex-app-server-error.jsonl", verified, want{"unknown", "receipt_artifact_tool_failed"}, want{"unknown", "receipt_artifact_missing_revision"}},
		{"codex-app-server-no-catalog.jsonl", want{"unknown", "receipt_artifact_missing_evidence"}, verified, want{"unknown", "receipt_artifact_missing_revision"}},
		{"codex-steps.jsonl", verified, verified, want{"unknown", "receipt_artifact_missing_revision"}},
		{"codex-steps-error.jsonl", verified, want{"unknown", "receipt_artifact_tool_failed"}, want{"unknown", "receipt_artifact_missing_revision"}},
		{"omo-transport.jsonl", verified, verified, verified},
		{"omo-transport-error.jsonl", want{"unknown", "receipt_artifact_missing_evidence"}, want{"unknown", "receipt_artifact_tool_failed"}, want{"unknown", "receipt_artifact_missing_revision"}},
		{"omo-revision-mentioned-only.jsonl", verified, verified, want{"unknown", "receipt_artifact_missing_revision"}},
		{"failed-call.jsonl", want{"unknown", "receipt_artifact_missing_evidence"}, want{"unknown", "receipt_artifact_missing_evidence"}, want{"unknown", "receipt_artifact_missing_revision"}},
	}
	for _, tc := range cases {
		t.Run(tc.fixture, func(t *testing.T) {
			fixture := newHostFixture(t)
			fixture.versions["codex"] = "0.128.0"
			artifact, err := filepath.Abs(filepath.Join("testdata", "receipts", tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			receipts := receiptFixture{dir: t.TempDir(), artifact: artifact}
			path := receipts.write(t, receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0"))

			codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

			requireStatus(t, "discovered", codex.Discovered, tc.discovered.status, tc.discovered.reason)
			requireStatus(t, "connected", codex.Connected, tc.connected.status, tc.connected.reason)
			requireStatus(t, "protocol", codex.Protocol, tc.protocol.status, tc.protocol.reason)
		})
	}
}
