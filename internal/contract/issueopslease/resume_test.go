package issueopslease

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestResumeReceiptJSONKeepsOnlySealedArtifactReferences(t *testing.T) {
	want := ResumeReceipt{
		Execution: Execution{Mode: "orca"},
		Artifacts: ResumeArtifacts{
			ClaimTokenPath:      "/worktree/lease-1.token",
			IssueBodySHA256:     "issue-sha",
			ContextPacketPath:   "/worktree/context.json",
			ContextPacketSHA256: "packet-sha",
			OwnerPromptPath:     "/worktree/owner-prompt.txt",
			OwnerPromptSHA256:   "prompt-sha",
		},
	}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["execution"] == nil || fields["artifacts"] == nil {
		t.Fatalf("resume receipt fields=%s", data)
	}
	if bytes.Contains(data, []byte("\"claim_token\"")) {
		t.Fatalf("resume receipt leaked a claim token: %s", data)
	}
}

func TestResumeStageReceiptJSONPreservesRunIdentity(t *testing.T) {
	want := ResumeStageReceipt{RunID: "run-resume", RunBound: true}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got ResumeStageReceipt
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Run stage receipt round-trip=%#v want=%#v", got, want)
	}
}

func TestOrcaPromptReceiptJSONPreservesBaselineWorkingSequencePresence(t *testing.T) {
	for _, test := range []struct {
		name        string
		baseline    string
		wantPresent bool
	}{
		{name: "missing"},
		{name: "explicit zero", baseline: `,"baseline_working_sequence":0`, wantPresent: true},
		{name: "positive", baseline: `,"baseline_working_sequence":9`, wantPresent: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw := []byte(`{"prompt_receipt":{"request_id":"22222222-2222-4222-8222-222222222222"` + test.baseline + `}}`)
			for _, contract := range []struct {
				name      string
				roundTrip func([]byte) ([]byte, error)
			}{
				{name: "resume", roundTrip: func(data []byte) ([]byte, error) {
					var receipt ResumeStageReceipt
					if err := json.Unmarshal(data, &receipt); err != nil {
						return nil, err
					}
					return json.Marshal(receipt)
				}},
				{name: "reconcile", roundTrip: func(data []byte) ([]byte, error) {
					var receipt ReconcileStageReceipt
					if err := json.Unmarshal(data, &receipt); err != nil {
						return nil, err
					}
					return json.Marshal(receipt)
				}},
			} {
				t.Run(contract.name, func(t *testing.T) {
					encoded, err := contract.roundTrip(raw)
					if err != nil {
						t.Fatal(err)
					}
					var outer map[string]json.RawMessage
					if err := json.Unmarshal(encoded, &outer); err != nil {
						t.Fatal(err)
					}
					var prompt map[string]json.RawMessage
					if err := json.Unmarshal(outer["prompt_receipt"], &prompt); err != nil {
						t.Fatal(err)
					}
					_, present := prompt["baseline_working_sequence"]
					if present != test.wantPresent {
						t.Fatalf("baseline presence=%v want=%v JSON=%s", present, test.wantPresent, encoded)
					}
				})
			}
		})
	}
}
