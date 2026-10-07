package issueopslease_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	leasecodec "issueops/internal/adapter/outbound/issueopsrecord"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

func TestValidateActorKeepsPublicText(t *testing.T) {
	for _, tc := range []struct {
		name  string
		actor leasecontract.Actor
		want  string
	}{
		{name: "invalid host", actor: leasecontract.Actor{Host: "other"}, want: "native actor host must be codex, claude, or omo"},
		{name: "missing session", actor: leasecontract.Actor{Host: "codex"}, want: "native actor session_id is required"},
		{name: "missing receipt", actor: leasecontract.Actor{Host: "codex", SessionID: "session"}, want: "native actor requires a PID reuse-safe session_process receipt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := leasedomain.ValidatePersistedActor(tc.actor); err == nil || err.Error() != tc.want {
				t.Fatalf("validateActor error=%v want=%q", err, tc.want)
			}
		})
	}
}

func TestValidateActorAcceptsOmo(t *testing.T) {
	err := leasedomain.ValidatePersistedActor(leasecontract.Actor{
		Host:      "omo",
		SessionID: "019ff5b8-7d62-707a-a693-5e7a5e8a3187",
		SessionProcess: &leasecontract.ProcessReceipt{
			PID: 42, StartedAt: "2026-08-12T00:00:00Z", Executable: "/Users/test/Library/pnpm/bin/omo",
		},
	})
	if err != nil {
		t.Fatalf("Omo native actor must be valid: %v", err)
	}
}

func TestValidateSidecarsRequiresVersionedOrcaArtifactIdentity(t *testing.T) {
	binding := leasecontract.OrcaBinding{
		RuntimeID: "runtime", RepoID: "repo", WorktreeID: "worktree", RunID: "run_issueops_1", LeaseGeneration: 1,
		OwnerHost: "codex", OwnerModel: "model", TaskID: "task", DispatchID: "dispatch",
	}
	execution := leasecontract.Execution{Mode: "orca", Orca: &binding, Selection: leaseSelectionFixture("orca")}
	if err := leasedomain.ValidatePersistedSidecars(execution); err == nil || !strings.Contains(err.Error(), "unsupported Orca artifact identity version 0") {
		t.Fatalf("unversioned all-empty identity must fail closed: %v", err)
	}

	execution.Orca.ArtifactIdentityVersion = leasecontract.OrcaArtifactIdentityVersion
	if err := leasedomain.ValidatePersistedSidecars(execution); err == nil || !strings.Contains(err.Error(), "version requires a complete sealed artifact identity") {
		t.Fatalf("versioned all-empty identity must fail as an invariant violation: %v", err)
	}

	execution.Orca.IssueBodySHA256 = strings.Repeat("a", 64)
	execution.Orca.ContextPacketSHA256 = strings.Repeat("b", 64)
	execution.Orca.OwnerPromptSHA256 = strings.Repeat("c", 64)
	if err := leasedomain.ValidatePersistedSidecars(execution); err != nil {
		t.Fatalf("versioned complete identity must be valid: %v", err)
	}
	for name, mutate := range map[string]func(*leasecontract.OrcaBinding){
		"run_id":           func(binding *leasecontract.OrcaBinding) { binding.RunID = "" },
		"lease_generation": func(binding *leasecontract.OrcaBinding) { binding.LeaseGeneration = 0 },
	} {
		missing := *execution.Orca
		mutate(&missing)
		if err := leasedomain.ValidatePersistedSidecars(leasecontract.Execution{Mode: "orca", Orca: &missing, Selection: leaseSelectionFixture("orca")}); err == nil || !strings.Contains(err.Error(), name) {
			t.Fatalf("binding without %s must fail closed: %v", name, err)
		}
	}
}

func TestCompletionHistoryStrictRoundTripAndRejectsUnstampedCompletion(t *testing.T) {
	record := completionHistoryRecord()
	encoded, err := leasecodec.EncodeLease(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := leasecodec.DecodeLease(record.ID, encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded.Execution.CompletionHistory, record.Execution.CompletionHistory) {
		t.Fatalf("completion history round trip=%+v want=%+v", decoded.Execution.CompletionHistory, record.Execution.CompletionHistory)
	}
	if decoded.Execution.CompletionHistory[0].Completion.Generation != 1 {
		t.Fatalf("completion generation did not round trip: %+v", decoded.Execution.CompletionHistory[0])
	}

	var unstamped map[string]any
	if err := json.Unmarshal(encoded, &unstamped); err != nil {
		t.Fatal(err)
	}
	execution := unstamped["execution"].(map[string]any)
	delete(execution, "completion_history")
	execution["completion"] = map[string]any{
		"final_head": strings.Repeat("c", 40), "verification_report_path": ".issueops/verified-execution/unstamped.json",
		"verification": []any{"go test ./... -count=1"}, "remote_artifact_url": "https://github.com/acme/repo/pull/1", "completed_at": "2026-08-02T00:00:00Z",
	}
	unstampedBytes, err := json.Marshal(unstamped)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leasecodec.DecodeLease(record.ID, unstampedBytes); err == nil {
		t.Fatal("a completion without its generation must fail closed")
	}
}

func TestCompletionHistoryRejectsIncompleteEntries(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*leasecontract.CompletionHistoryEntry)
	}{
		{name: "generation", mutate: func(entry *leasecontract.CompletionHistoryEntry) { entry.Generation = 0 }},
		{name: "completion", mutate: func(entry *leasecontract.CompletionHistoryEntry) { entry.Completion.Verification = nil }},
		{name: "blank verification", mutate: func(entry *leasecontract.CompletionHistoryEntry) { entry.Completion.Verification = []string{" "} }},
		{name: "current generation", mutate: func(entry *leasecontract.CompletionHistoryEntry) {
			entry.Generation = 2
			entry.Completion.Generation = 2
		}},
		{name: "future generation", mutate: func(entry *leasecontract.CompletionHistoryEntry) {
			entry.Generation = 3
			entry.Completion.Generation = 3
		}},
		{name: "reason", mutate: func(entry *leasecontract.CompletionHistoryEntry) { entry.Reason = " " }},
		{name: "reopened at", mutate: func(entry *leasecontract.CompletionHistoryEntry) { entry.ReopenedAt = " " }},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := completionHistoryRecord()
			test.mutate(&record.Execution.CompletionHistory[0])
			if _, err := leasecodec.EncodeLease(record); err == nil {
				t.Fatal("invalid completion history accepted")
			}
		})
	}
}

func TestCurrentCompletionRejectsBlankVerificationEvidence(t *testing.T) {
	record := completionHistoryRecord()
	record.Execution.CompletionHistory = nil
	record.Execution.Completion = &leasecontract.Completion{
		FinalHead: strings.Repeat("b", 40), VerificationReportPath: ".issueops/verified-execution/report.json",
		Verification: []string{" "}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	if _, err := leasecodec.EncodeLease(record); err == nil {
		t.Fatal("current completion with blank verification evidence accepted")
	}
}

func TestCompletionGenerationValidation(t *testing.T) {
	record := completionHistoryRecord()
	record.Execution.CompletionHistory[0].Completion.Generation = 2
	if _, err := leasecodec.EncodeLease(record); err == nil || !strings.Contains(err.Error(), "generation conflicts") {
		t.Fatalf("history generation conflict error=%v", err)
	}

	record = completionHistoryRecord()
	record.Execution.CompletionHistory = nil
	record.Execution.Completion = &leasecontract.Completion{
		Generation: 3, FinalHead: strings.Repeat("b", 40), VerificationReportPath: ".issueops/verified-execution/report.json",
		Verification: []string{"go test ./..."}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z",
	}
	if _, err := leasecodec.EncodeLease(record); err == nil || !strings.Contains(err.Error(), "exceeds the lease generation") {
		t.Fatalf("current completion generation error=%v", err)
	}
}

func completionHistoryRecord() leasecontract.Record {
	return leasecontract.Record{
		SchemaVersion: leasecontract.SchemaVersion,
		ID:            "io-completion-history",
		Execution: &leasecontract.Execution{
			Mode:      "direct",
			Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z"},
			Lease:     leasecontract.Lease{Generation: 2, Status: "released"},
			CompletionHistory: []leasecontract.CompletionHistoryEntry{{
				Generation: 1,
				Completion: leasecontract.Completion{Generation: 1, FinalHead: strings.Repeat("b", 40), VerificationReportPath: ".issueops/verified-execution/report.json", Verification: []string{"go test ./... -count=1"}, RemoteArtifactURL: "https://github.com/acme/repo/pull/1", CompletedAt: "2026-08-03T00:00:00Z"},
				Reason:     "new verified HEAD", ReopenedAt: "2026-08-04T00:00:00Z",
			}},
			Selection: leaseSelectionFixture("direct"),
		},
	}
}

func TestSelectionReceiptRejectsExplicitDirectFallbackCode(t *testing.T) {
	selection := leasecontract.Selection{
		RequestedMode: "direct", ResolvedMode: "direct",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:01Z",
		ExplicitDirectReason: "manual recovery",
	}
	for _, fallback := range []string{"orca_unready", " "} {
		selection.FallbackCode = fallback
		if err := leasedomain.ValidatePersistedSelection(selection, "direct"); err == nil {
			t.Fatalf("explicit direct selection with fallback_code %q accepted", fallback)
		}
	}
}

func assertJSONShape(t *testing.T, source, target reflect.Type, path string) {
	t.Helper()
	source = dereferenceJSONType(source)
	target = dereferenceJSONType(target)
	if source.Kind() == reflect.Slice || source.Kind() == reflect.Array {
		if target.Kind() != source.Kind() {
			t.Fatalf("%s target kind=%s want=%s", path, target.Kind(), source.Kind())
		}
		assertJSONShape(t, source.Elem(), target.Elem(), path+"[]")
		return
	}
	if source.Kind() == reflect.Map {
		if target.Kind() != reflect.Map {
			t.Fatalf("%s target kind=%s want=map", path, target.Kind())
		}
		assertJSONShape(t, source.Elem(), target.Elem(), path+"{}")
		return
	}
	if source.Kind() != reflect.Struct {
		return
	}
	targetFields := jsonTaggedFields(target)
	for _, field := range jsonTaggedFields(source) {
		candidate, ok := targetFields[field.tag]
		if !ok {
			t.Fatalf("%s.%s (%s) is absent from the lease shape", path, field.tag, field.typ)
		}
		assertJSONShape(t, field.typ, candidate.typ, path+"."+field.tag)
	}
}

type jsonTaggedField struct {
	tag string
	typ reflect.Type
}

func jsonTaggedFields(typ reflect.Type) map[string]jsonTaggedField {
	fields := map[string]jsonTaggedField{}
	for index := 0; index < typ.NumField(); index++ {
		field := typ.Field(index)
		if !field.IsExported() {
			continue
		}
		tag := strings.Split(field.Tag.Get("json"), ",")[0]
		if tag == "" || tag == "-" {
			continue
		}
		fields[tag] = jsonTaggedField{tag: tag, typ: field.Type}
	}
	return fields
}

func dereferenceJSONType(typ reflect.Type) reflect.Type {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	return typ
}
func TestSelectionReceiptRoundTripsAndIsRequiredInCurrentV1(t *testing.T) {
	record := leasecontract.Record{
		SchemaVersion: leasecontract.SchemaVersion,
		ID:            "io-selection",
		Execution: &leasecontract.Execution{
			Mode: "direct",
			Workspace: leasecontract.Workspace{
				SourceRoot: "/repo", Root: "/repo.worktrees/selection", Branch: "selection",
				BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z",
			},
			Lease: leasecontract.Lease{Generation: 1, Status: "released"},
		},
	}
	if _, err := leasecodec.EncodeLease(record); err == nil || !strings.Contains(err.Error(), "invalid state") && !strings.Contains(err.Error(), "selection receipt is required") {
		t.Fatalf("an execution without a selection receipt must not be encoded: %v", err)
	}
	unvalidated, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := leasecodec.DecodeLease(record.ID, unvalidated); err == nil {
		t.Fatal("an execution without a selection receipt must not be decoded")
	}

	record.Execution.Selection = &leasecontract.Selection{
		RequestedMode: "direct", ResolvedMode: "direct",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:01Z",
		ExplicitDirectReason: "manual recovery",
	}
	encoded, err := leasecodec.EncodeLease(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := leasecodec.DecodeLease(record.ID, encoded)
	if err != nil || !reflect.DeepEqual(decoded.Execution.Selection, record.Execution.Selection) {
		t.Fatalf("selection receipt changed across current-v1 round trip: got=%+v want=%+v err=%v", decoded.Execution.Selection, record.Execution.Selection, err)
	}
}

func TestSelectionReceiptRequiresExactAutoFallbackCode(t *testing.T) {
	selection := leasecontract.Selection{
		RequestedMode: "auto", ResolvedMode: "direct", ProbeAttempted: true,
		ProbeCode: "orca_unready", FallbackCode: "orca_unready",
		ReadinessFingerprint: strings.Repeat("b", 64), SelectedAt: "2026-08-03T00:00:01Z",
	}
	if err := leasedomain.ValidatePersistedSelection(selection, "direct"); err != nil {
		t.Fatalf("valid auto fallback rejected: %v", err)
	}
	for _, fallback := range []string{"", "different_code", " orca_unready "} {
		candidate := selection
		candidate.FallbackCode = fallback
		if err := leasedomain.ValidatePersistedSelection(candidate, "direct"); err == nil {
			t.Fatalf("invalid fallback_code %q accepted", fallback)
		}
	}
}

func TestDevilsAdvocateReviewBindingFieldsRoundTrip(t *testing.T) {
	record := completionHistoryRecord()
	record.DevilsAdvocateReview = json.RawMessage(`{"verdict":"pass","findings":["attacked gate 3"],"reviewer_pattern":"devils-advocate-review","reviewer_context":"subagent","reviewed_plan_digest":"` + strings.Repeat("d", 64) + `","history":[{"verdict":"revise","findings":["gate 1"],"reviewer_context":"subagent","reviewed_plan_digest":"` + strings.Repeat("e", 64) + `","recorded_at":"2026-08-28T00:00:00Z"}],"recorded_at":"2026-08-28T00:01:00Z"}`)
	encoded, err := leasecodec.EncodeLease(record)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := leasecodec.DecodeLease(record.ID, encoded)
	if err != nil {
		t.Fatalf("plan-bound devil's-advocate review must stay readable by the lease decoder: %v", err)
	}
	var review struct {
		ReviewerContext    string `json:"reviewer_context"`
		ReviewedPlanDigest string `json:"reviewed_plan_digest"`
		History            []struct {
			Verdict string `json:"verdict"`
		} `json:"history"`
	}
	if err := json.Unmarshal(decoded.DevilsAdvocateReview, &review); err != nil {
		t.Fatal(err)
	}
	if review.ReviewerContext != "subagent" || review.ReviewedPlanDigest != strings.Repeat("d", 64) || len(review.History) != 1 || review.History[0].Verdict != "revise" {
		t.Fatalf("binding fields did not round trip: %+v", review)
	}
}
