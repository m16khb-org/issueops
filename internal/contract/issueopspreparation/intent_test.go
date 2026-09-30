package issueopspreparation

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

const (
	prepareOperationID = "0123456789abcdef0123456789abcdef"
	resumeOperationID  = "fedcba9876543210fedcba9876543210"
	digestA            = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB            = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	prepareIntentJSON = `{"schema_version":1,"purpose":"prepare","operation_id":"0123456789abcdef0123456789abcdef","lifecycle_id":"io-codec-prepare","generation":1,"stage":"worktree_create","marker":"issueops-v1 lifecycle=io-codec-prepare operation=0123456789abcdef0123456789abcdef provider=github issue=199","started_at":"2026-08-02T00:00:00Z","invocation_state":"not_invoked_proven","invocation_attempts":0,"workspace":{"lifecycle_id":"io-codec-prepare","source_root":"/repo","root":"/repo.worktrees/199-prepare","branch":"199-prepare","base_branch":"117-parent","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm":true},"probe":{"repo":"/repo","host":"codex","model":"gpt-6-sol","effort":"high","provider":"github","issue":199,"marker":"issueops-v1 lifecycle=io-codec-prepare operation=0123456789abcdef0123456789abcdef provider=github issue=199"},"issue_body_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`

	resumeIntentJSON = `{"schema_version":1,"purpose":"resume","operation_id":"fedcba9876543210fedcba9876543210","lifecycle_id":"io-codec-resume","generation":2,"stage":"terminal_create","marker":"issueops-v1 resume lifecycle=io-codec-resume generation=2 operation=fedcba9876543210fedcba9876543210 provider=github issue=199","started_at":"2026-08-02T00:01:00Z","invocation_state":"not_invoked_proven","invocation_attempts":0,"workspace":{"lifecycle_id":"io-codec-resume","source_root":"/repo","root":"/repo.worktrees/199-resume","branch":"199-resume","base_branch":"117-parent","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","confirm":true},"probe":{"repo":"/repo","host":"claude","model":"claude-opus-5-5","effort":"high","provider":"github","issue":199,"marker":"issueops-v1 resume lifecycle=io-codec-resume generation=2 operation=fedcba9876543210fedcba9876543210 provider=github issue=199"},"prepared":{"workspace":{"source_root":"/repo","root":"/repo.worktrees/199-resume","branch":"199-resume","base_head":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","driver":"orca"},"runtime_id":"runtime","repo_id":"repo","worktree_id":"worktree","worktree_instance_id":"instance"},"launch":{"prompt_path":"/repo.worktrees/199-resume/.issueops/owner.md","prompt_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","context_packet_path":"/repo.worktrees/199-resume/.issueops/context.json","context_packet_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},"issue_body_sha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","claim_token_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","prior_binding":{"runtime_id":"runtime","repo_id":"repo","worktree_id":"worktree","worktree_instance_id":"instance","lease_generation":1,"owner_host":"claude","owner_model":"claude-opus-5-5","owner_effort":"high","run_id":"run-old","task_id":"task-old","dispatch_id":"dispatch-old","terminal_pty_id":"terminal-old"},"resume_lease":{"generation":2,"status":"claimable","claim_token_sha256":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"}}`
)

func TestIntentCodecRoundTripsPrepareAndResumeBytes(t *testing.T) {
	codec := IntentCodec{}
	for _, test := range []struct {
		name        string
		operationID string
		raw         string
	}{
		{name: "prepare", operationID: prepareOperationID, raw: prepareIntentJSON},
		{name: "resume", operationID: resumeOperationID, raw: resumeIntentJSON},
	} {
		t.Run(test.name, func(t *testing.T) {
			decoded, err := codec.Decode(test.operationID, []byte(test.raw))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			encoded, err := codec.Encode(decoded)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if !bytes.Equal(encoded, []byte(test.raw)) {
				t.Fatalf("intent bytes changed\nwant=%s\n got=%s", test.raw, encoded)
			}
		})
	}
}

func TestIntentCodecDecodesSelfIdentifiedPayloadForAuthorityCheck(t *testing.T) {
	codec := IntentCodec{}
	intent, err := codec.DecodeSelfIdentified([]byte(prepareIntentJSON))
	if err != nil || intent.OperationID != prepareOperationID {
		t.Fatalf("self-identified intent = %+v, %v", intent, err)
	}
	if _, err := codec.DecodeSelfIdentified(retiredPrepareIntentBytes(t)); err == nil || !strings.Contains(err.Error(), "intent_marker_invalid") {
		t.Fatalf("retired marker accepted: %v", err)
	}
}

func TestIntentCodecAcceptsOmoOwner(t *testing.T) {
	var intent Intent
	if err := json.Unmarshal([]byte(prepareIntentJSON), &intent); err != nil {
		t.Fatal(err)
	}
	intent.Probe.Host = "omo"
	intent.Probe.Model = "chatgpt-subscription/gpt-6-sol"
	intent.Probe.Effort = "max"
	if err := (IntentCodec{}).Validate(intent, prepareOperationID); err != nil {
		t.Fatalf("Omo owner intent must be valid: %v", err)
	}
}

func TestPromptReceiptJSONPreservesBaselineWorkingSequencePresence(t *testing.T) {
	for _, test := range []struct {
		name        string
		raw         string
		wantPresent bool
		wantValue   uint64
	}{
		{name: "missing", raw: `{"request_id":"22222222-2222-4222-8222-222222222222"}`},
		{name: "explicit zero", raw: `{"request_id":"22222222-2222-4222-8222-222222222222","baseline_working_sequence":0}`, wantPresent: true},
		{name: "positive", raw: `{"request_id":"22222222-2222-4222-8222-222222222222","baseline_working_sequence":9}`, wantPresent: true, wantValue: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			var receipt PromptReceipt
			if err := json.Unmarshal([]byte(test.raw), &receipt); err != nil {
				t.Fatal(err)
			}
			encoded, err := json.Marshal(receipt)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			value, present := fields["baseline_working_sequence"]
			if present != test.wantPresent {
				t.Fatalf("baseline presence=%v want=%v JSON=%s", present, test.wantPresent, encoded)
			}
			if present && string(value) != strconv.FormatUint(test.wantValue, 10) {
				t.Fatalf("baseline value=%s want=%d JSON=%s", value, test.wantValue, encoded)
			}
		})
	}
}

func TestIntentCodecRejectsIdentityAndAuthorityDrift(t *testing.T) {
	codec := IntentCodec{}
	tests := []struct {
		name        string
		operationID string
		raw         string
		want        string
	}{
		{name: "operation", operationID: resumeOperationID, raw: prepareIntentJSON, want: "Orca external intent payload is invalid"},
		{name: "prepare generation", operationID: prepareOperationID, raw: strings.Replace(prepareIntentJSON, `"generation":1`, `"generation":2`, 1), want: "Orca prepare intent payload is invalid"},
		{name: "resume authority", operationID: resumeOperationID, raw: strings.Replace(resumeIntentJSON, `,"resume_lease":{"generation":2,"status":"claimable","claim_token_sha256":"`+digestB+`"}`, "", 1), want: "Orca resume intent payload is invalid"},
		{name: "marker issue", operationID: prepareOperationID, raw: strings.ReplaceAll(prepareIntentJSON, "provider=github issue=199", "provider=github issue=200"), want: "intent_identity_mismatch"},
		{name: "attempt bound", operationID: prepareOperationID, raw: strings.Replace(prepareIntentJSON, `"invocation_attempts":0`, `"invocation_attempts":3`, 1), want: "Orca external intent payload is invalid"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := codec.Decode(test.operationID, []byte(test.raw))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("err=%v want substring %q", err, test.want)
			}
		})
	}
}

func TestIntentCodecRejectsNonCanonicalMarkerTokensAndPrepareGeneration(t *testing.T) {
	codec := IntentCodec{}
	for _, identity := range []MarkerIdentity{
		{Purpose: PurposePrepare, LifecycleID: "io-prepare", Generation: 2, OperationID: prepareOperationID, Provider: "github", Issue: 199},
		{Purpose: PurposePrepare, LifecycleID: "io-prepare\u00a0suffix", Generation: 1, OperationID: prepareOperationID, Provider: "github", Issue: 199},
	} {
		if marker, err := codec.RenderMarker(identity); err == nil {
			t.Fatalf("non-canonical identity rendered marker %q", marker)
		}
	}
}

func TestIntentCodecRejectsRetiredMarkerWithoutMutation(t *testing.T) {
	codec := IntentCodec{}
	raw := retiredPrepareIntentBytes(t)
	beforeRaw := append([]byte(nil), raw...)
	_, err := codec.Decode(prepareOperationID, raw)
	if err == nil || !strings.Contains(err.Error(), "intent_marker_invalid") {
		t.Fatalf("decode retired marker: %v", err)
	}
	if !bytes.Equal(raw, beforeRaw) {
		t.Fatal("retired marker rejection mutated the input bytes")
	}
}

func TestRenderReadinessMarker(t *testing.T) {
	codec := IntentCodec{}
	marker, err := codec.RenderReadinessMarker("io-prepare", IssueIdentity{Provider: "github", Issue: 199})
	if err != nil {
		t.Fatal(err)
	}
	if marker != "issueops-v1 lifecycle=io-prepare provider=github issue=199" {
		t.Fatalf("marker=%q", marker)
	}
}

func TestDecodeIssueLinkEvidencePreservesDefinedSidecarFields(t *testing.T) {
	raw := []byte(`{"provider":"gitlab","issue_url":"https://gitlab.com/example/repo/-/work_items/199","link_verified":true,"base_head":"abc"}`)
	link := DecodeIssueLinkEvidence(raw)
	if link == nil || link.Provider != "gitlab" || !link.LinkVerified || link.IssueURL == "" {
		t.Fatalf("decoded link = %+v", link)
	}
	if DecodeIssueLinkEvidence([]byte(`{"provider":"gitlab"} broken`)) != nil {
		t.Fatal("malformed branch-prepare sidecar accepted")
	}
}

func retiredPrepareIntentBytes(t *testing.T) []byte {
	t.Helper()
	canonical := "issueops-v1 lifecycle=io-codec-prepare operation=" + prepareOperationID + " provider=github issue=199"
	retired := "issueops-v1 lifecycle=io-codec-prepare operation=" + prepareOperationID
	return []byte(strings.ReplaceAll(prepareIntentJSON, canonical, retired))
}
