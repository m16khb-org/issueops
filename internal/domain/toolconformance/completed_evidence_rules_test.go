package toolconformance

import (
	"strings"
	"testing"

	failurecausecontract "issueops/internal/contract/failurecause"
	contract "issueops/internal/contract/toolconformance"
	failurecausedomain "issueops/internal/domain/failurecause"
)

func testExpectation() CompletedEpisodeExpectation {
	return CompletedEpisodeExpectation{
		Host: "codex", HostVersion: "1.0", RequestedModel: "req", Profile: "p",
		Fixture: contract.Fixture{ID: "f1", SchemaSHA256: "schema"}, Attempt: 1,
	}
}

// validEpisode returns an exact_valid completed episode matching testExpectation.
func validEpisode() contract.EpisodeReport {
	e := contract.EpisodeReport{
		Status: contract.EpisodeCompleted, Host: "codex", HostVersion: "1.0", RequestedModel: "req", ObservedModel: "obs",
		FixtureID: "f1", SchemaSHA256: "schema", Profile: "p", Attempt: 1, DurationMS: 5,
		SessionStartObserved: true, AmbientToolCount: 1, CallCount: 1,
		ResponseSHA256: evidenceID("1"), RawArgumentsSHA256: evidenceID("2"), EvidenceID: evidenceID("3"),
		CanonicalArguments: map[string]any{}, Classification: contract.Classification(contract.ExactValid),
		AdvertisedValid: true, CanonicalValid: true, Diagnostics: []contract.Diagnostic{},
		FailureCause: failurecausecontract.None, FailureCauseReason: "no_failed_steps",
		FailureCauseEvidence: []failurecausecontract.Evidence{},
	}
	e.DiagnosticSignature = DiagnosticSignature(e.Classification, e.Diagnostics)
	return e
}

// failedEpisode returns an unknown_key episode whose failure cause is derived like production does.
func failedEpisode(advertisedValid, canonicalValid bool, cause failurecausecontract.Cause) contract.EpisodeReport {
	e := validEpisode()
	e.Classification = contract.Classification(contract.UnknownKey)
	e.AdvertisedValid, e.CanonicalValid = advertisedValid, canonicalValid
	e.Diagnostics = []contract.Diagnostic{{Path: "/x", Code: "unknown_key"}}
	e.DiagnosticSignature = DiagnosticSignature(e.Classification, e.Diagnostics)
	e.FailureCauseEvidence = []failurecausecontract.Evidence{{Cause: cause, Code: string(e.Classification), Source: "tool_conformance"}}
	result := failurecausedomain.Classify(true, e.FailureCauseEvidence)
	e.FailureCause, e.FailureCauseReason, e.FailureCauseEvidence = result.Cause, result.Reason, result.Evidence
	return e
}

func TestValidCompletedEpisodeAcceptsConsistentEpisodes(t *testing.T) {
	expected := testExpectation()
	for name, episode := range map[string]contract.EpisodeReport{
		"exact_valid":                validEpisode(),
		"drift_model_cause":          failedEpisode(false, false, failurecausecontract.Model),
		"drift_contract_input_cause": failedEpisode(true, false, failurecausecontract.ContractInput),
	} {
		if !ValidCompletedEpisode(episode, expected) {
			t.Errorf("%s: consistent episode rejected", name)
		}
	}
}

func TestValidCompletedEpisodeRejectsInconsistentEvidence(t *testing.T) {
	expected := testExpectation()
	tooLong := strings.Repeat("x", 1025)
	cases := map[string]func(e *contract.EpisodeReport){
		"incomplete status":           func(e *contract.EpisodeReport) { e.Status = contract.EpisodeIncomplete },
		"host mismatch":               func(e *contract.EpisodeReport) { e.Host = "claude" },
		"host version mismatch":       func(e *contract.EpisodeReport) { e.HostVersion = "2" },
		"requested model mismatch":    func(e *contract.EpisodeReport) { e.RequestedModel = "other" },
		"schema mismatch":             func(e *contract.EpisodeReport) { e.SchemaSHA256 = "other" },
		"attempt mismatch":            func(e *contract.EpisodeReport) { e.Attempt = 2 },
		"blank observed model":        func(e *contract.EpisodeReport) { e.ObservedModel = "  " },
		"padded observed model":       func(e *contract.EpisodeReport) { e.ObservedModel = " obs" },
		"multiline observed model":    func(e *contract.EpisodeReport) { e.ObservedModel = "a\nb" },
		"oversized observed model":    func(e *contract.EpisodeReport) { e.ObservedModel = strings.Repeat("m", 257) },
		"zero duration":               func(e *contract.EpisodeReport) { e.DurationMS = 0 },
		"session start missing":       func(e *contract.EpisodeReport) { e.SessionStartObserved = false },
		"pre tool use blocked":        func(e *contract.EpisodeReport) { e.PreToolUseObserved = true },
		"nonzero exit":                func(e *contract.EpisodeReport) { e.ExitCode = 1 },
		"extra ambient tools":         func(e *contract.EpisodeReport) { e.AmbientToolCount = 2 },
		"multiple calls counted":      func(e *contract.EpisodeReport) { e.CallCount = 2 },
		"short evidence id":           func(e *contract.EpisodeReport) { e.EvidenceID = "abc" },
		"non-hex raw arguments hash":  func(e *contract.EpisodeReport) { e.RawArgumentsSHA256 = strings.Repeat("z", 64) },
		"short response hash":         func(e *contract.EpisodeReport) { e.ResponseSHA256 = "" },
		"missing canonical arguments": func(e *contract.EpisodeReport) { e.CanonicalArguments = nil },
		"stale diagnostic signature":  func(e *contract.EpisodeReport) { e.DiagnosticSignature = evidenceID("9") },
		"exact_valid with diagnostics": func(e *contract.EpisodeReport) {
			e.Diagnostics = []contract.Diagnostic{{Path: "/a", Code: "c"}}
			e.DiagnosticSignature = DiagnosticSignature(e.Classification, e.Diagnostics)
		},
		"exact_valid not advertised valid": func(e *contract.EpisodeReport) { e.AdvertisedValid = false },
		"exact_valid not canonical valid":  func(e *contract.EpisodeReport) { e.CanonicalValid = false },
		"exact_valid nil failure evidence": func(e *contract.EpisodeReport) { e.FailureCauseEvidence = nil },
		"exact_valid with failure evidence": func(e *contract.EpisodeReport) {
			e.FailureCauseEvidence = []failurecausecontract.Evidence{{Cause: failurecausecontract.Model, Code: "x", Source: "tool_conformance"}}
		},
		"exact_valid wrong cause":  func(e *contract.EpisodeReport) { e.FailureCause = failurecausecontract.Model },
		"exact_valid wrong reason": func(e *contract.EpisodeReport) { e.FailureCauseReason = "other" },
	}
	for name, mutate := range cases {
		episode := validEpisode()
		mutate(&episode)
		if ValidCompletedEpisode(episode, expected) {
			t.Errorf("%s: accepted", name)
		}
	}

	badDiagnostic := func(d contract.Diagnostic) contract.EpisodeReport {
		e := failedEpisode(false, false, failurecausedomainModel())
		e.Diagnostics = []contract.Diagnostic{d}
		e.DiagnosticSignature = DiagnosticSignature(e.Classification, e.Diagnostics)
		return e
	}
	for name, d := range map[string]contract.Diagnostic{
		"empty code":    {Path: "/x"},
		"long path":     {Path: strings.Repeat("p", 513), Code: "c"},
		"long code":     {Path: "/x", Code: strings.Repeat("c", 129)},
		"long expected": {Path: "/x", Code: "c", Expected: tooLong},
		"long actual":   {Path: "/x", Code: "c", Actual: tooLong},
	} {
		if ValidCompletedEpisode(badDiagnostic(d), expected) {
			t.Errorf("diagnostic %s: accepted", name)
		}
	}
}

func failurecausedomainModel() failurecausecontract.Cause { return failurecausecontract.Model }

func TestValidCompletedEpisodeRejectsUnsortedDiagnostics(t *testing.T) {
	e := failedEpisode(false, false, failurecausecontract.Model)
	e.Diagnostics = []contract.Diagnostic{{Path: "/b", Code: "c"}, {Path: "/a", Code: "c"}}
	e.DiagnosticSignature = DiagnosticSignature(e.Classification, e.Diagnostics)
	if ValidCompletedEpisode(e, testExpectation()) {
		t.Fatal("unsorted diagnostics accepted")
	}
}

func TestValidCompletedEpisodeRejectsSchemaDriftThatIsCanonicalValid(t *testing.T) {
	e := failedEpisode(true, true, failurecausecontract.Model)
	if ValidCompletedEpisode(e, testExpectation()) {
		t.Fatal("schema drift with canonical-valid arguments accepted")
	}
}

func TestValidCompletedEpisodeRequiresFailureCauseMatchingValidity(t *testing.T) {
	expected := testExpectation()
	for name, e := range map[string]contract.EpisodeReport{
		"model cause while advertised valid and canonical invalid":   failedEpisode(true, false, failurecausecontract.Model),
		"contract_input cause while canonical valid or unadvertised": failedEpisode(false, false, failurecausecontract.ContractInput),
	} {
		if ValidCompletedEpisode(e, expected) {
			t.Errorf("%s: accepted", name)
		}
	}
	e := failedEpisode(false, false, failurecausecontract.Model)
	e.FailureCauseEvidence[0].Source = "other"
	if ValidCompletedEpisode(e, expected) {
		t.Error("foreign evidence source accepted")
	}
	e = failedEpisode(false, false, failurecausecontract.Model)
	e.FailureCauseEvidence = append(e.FailureCauseEvidence, e.FailureCauseEvidence[0])
	if ValidCompletedEpisode(e, expected) {
		t.Error("multiple failure evidence entries accepted")
	}
	e = failedEpisode(false, false, failurecausecontract.Model)
	e.FailureCause = failurecausecontract.Unknown
	if ValidCompletedEpisode(e, expected) {
		t.Error("failure cause diverging from classification accepted")
	}
}

func TestValidCompletedEpisodeRejectsDisallowedClassificationsWithOtherwiseConsistentEvidence(t *testing.T) {
	for _, classification := range []contract.Classification{
		"bogus",
		contract.Classification(contract.NoCall),
		contract.Classification(contract.MultipleCalls),
		contract.Classification(contract.InvalidJSON),
	} {
		e := validEpisode()
		e.Classification = classification
		e.AdvertisedValid, e.CanonicalValid = false, false
		e.DiagnosticSignature = DiagnosticSignature(classification, e.Diagnostics)
		result := failurecausedomain.Classify(true, []failurecausecontract.Evidence{{Cause: failurecausecontract.Model, Code: string(classification), Source: "tool_conformance"}})
		e.FailureCause, e.FailureCauseReason, e.FailureCauseEvidence = result.Cause, result.Reason, result.Evidence
		if ValidCompletedEpisode(e, testExpectation()) {
			t.Errorf("classification %q accepted", classification)
		}
	}
}
