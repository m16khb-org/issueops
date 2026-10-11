package toolconformance

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	failurecausecontract "issueops/internal/contract/failurecause"
	contract "issueops/internal/contract/toolconformance"
	failurecausedomain "issueops/internal/domain/failurecause"
	"issueops/internal/port"
)

// scriptedRunner answers Run calls from a script and records every request.
type scriptedRunner struct {
	preflight port.HostProbePreflight
	script    func(call int, request port.HostProbeRequest) port.HostProbeResult
	requests  []port.HostProbeRequest
}

func (r *scriptedRunner) Name() string { return "scripted" }
func (r *scriptedRunner) Preflight(context.Context, port.HostProbeRequest) port.HostProbePreflight {
	return r.preflight
}
func (r *scriptedRunner) Run(_ context.Context, request port.HostProbeRequest) port.HostProbeResult {
	r.requests = append(r.requests, request)
	return r.script(len(r.requests), request)
}

func readyPreflight() port.HostProbePreflight {
	return port.HostProbePreflight{Ready: true, Installed: true, MockExtensionVerified: true, Version: "1.2.3"}
}

func hex64(c string) string { return strings.Repeat(c, 64) }

// validResult echoes the request into a result that passes completed-episode validation.
func validResult(host string, request port.HostProbeRequest, evidence string) port.HostProbeResult {
	return port.HostProbeResult{
		Completed: true, Host: host, HostVersion: request.HostVersion, RequestedModel: request.Model,
		ObservedModel: "observed-model", FixtureID: request.FixtureID, SchemaSHA256: request.SchemaSHA256,
		Profile: request.Profile, Attempt: request.Attempt, DurationMS: 5,
		SessionStartObserved: true, AmbientToolCount: 1, CallCount: 1,
		ResponseSHA256: hex64("b"), RawArgumentsSHA256: hex64("c"), CanonicalArgumentsJSON: `{"a":1}`,
		EvidenceID: evidence, Classification: contract.ExactValid, AdvertisedValid: true, CanonicalValid: true,
		DiagnosticsJSON: `[]`,
	}
}

func testFixtures() []contract.Fixture {
	return []contract.Fixture{
		{ID: "fx-b", ProbeTool: "probe_b", SourceTool: "tool_b", SchemaSHA256: hex64("2"), ExpectedArguments: map[string]any{"x": 1}},
		{ID: "fx-a", ProbeTool: "probe_a", SourceTool: "tool_a", SchemaSHA256: hex64("1"), ExpectedArguments: map[string]any{"y": "z"}},
	}
}

func liveDeps(runners map[string]port.HostProbeRunner, fixtures []contract.Fixture) LiveBenchmarkDependencies {
	tokens := 0
	return LiveBenchmarkDependencies{
		Runners: runners,
		Now:     func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC) },
		Token:   func() string { tokens++; return strings.Repeat("t", tokens) },
		LoadManifest: func([]contract.ToolDescriptor) ([]contract.Fixture, []contract.BaselineCase, error) {
			return fixtures, nil, nil
		},
		Classify: failurecausedomain.Classify,
	}
}

func baseRequest() LiveBenchmarkRequest {
	return LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", TargetCompleted: 1, MaxAttemptsPerCase: 1,
		HarnessBinary: "/bin/harness", RunID: "run-1",
	}
}

func evidenceByAttempt(host string) func(int, port.HostProbeRequest) port.HostProbeResult {
	return func(call int, request port.HostProbeRequest) port.HostProbeResult {
		return validResult(host, request, strings.Repeat(string(rune('a'+call)), 64))
	}
}

func TestValidateLiveRequestRejectsEachInvalidField(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*LiveBenchmarkRequest)
		want   string
	}{
		{"hosts", func(r *LiveBenchmarkRequest) { r.Hosts = nil }, "hosts_required"},
		{"profile", func(r *LiveBenchmarkRequest) { r.Profile = "noisy" }, "invalid_profile:noisy"},
		{"target", func(r *LiveBenchmarkRequest) { r.TargetCompleted = 5 }, "invalid_target_completed:5"},
		{"attempts low", func(r *LiveBenchmarkRequest) { r.MaxAttemptsPerCase = 0 }, "invalid_max_attempts_per_case:0"},
		{"attempts high", func(r *LiveBenchmarkRequest) { r.MaxAttemptsPerCase = 4 }, "invalid_max_attempts_per_case:4"},
		{"repro needs only", func(r *LiveBenchmarkRequest) { r.TargetCompleted = 10 }, "only_required_for_reproduction"},
		{"binary", func(r *LiveBenchmarkRequest) { r.HarnessBinary = "" }, "harness_binary_required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := baseRequest()
			tc.mutate(&request)
			if err := validateLiveRequest(request); err == nil || err.Error() != tc.want {
				t.Fatalf("err=%v want %s", err, tc.want)
			}
		})
	}
	for _, target := range []int{1, 10, 20} {
		request := baseRequest()
		request.TargetCompleted = target
		request.Only = "codex:fx-a"
		if err := validateLiveRequest(request); err != nil {
			t.Fatalf("target %d rejected: %v", target, err)
		}
	}
	request := baseRequest()
	request.Profile = "context-pressure"
	if err := validateLiveRequest(request); err != nil {
		t.Fatalf("context-pressure rejected: %v", err)
	}
}

func TestSelectFixturePairsOrderingAndOnlyFilter(t *testing.T) {
	fixtures := testFixtures()
	pairs, err := selectFixturePairs([]string{"codex", "claude"}, fixtures, "")
	if err != nil || len(pairs) != 4 {
		t.Fatalf("pairs=%+v err=%v", pairs, err)
	}
	if pairs[0].Host != "codex" || pairs[0].Fixture.ID != "fx-b" || pairs[3].Host != "claude" || pairs[3].Fixture.ID != "fx-a" {
		t.Fatalf("host-major manifest order lost: %+v", pairs)
	}
	pairs, err = selectFixturePairs([]string{"codex", "claude"}, fixtures, "claude:fx-b")
	if err != nil || len(pairs) != 1 || pairs[0].Host != "claude" || pairs[0].Fixture.ID != "fx-b" {
		t.Fatalf("pairs=%+v err=%v", pairs, err)
	}
	for _, only := range []string{"claude", ":fx-a", "claude:", "a:b:c"} {
		if _, err := selectFixturePairs([]string{"claude"}, fixtures, only); err == nil || err.Error() != "invalid_only:"+only {
			t.Fatalf("only=%q err=%v", only, err)
		}
	}
	if _, err := selectFixturePairs([]string{"codex"}, fixtures, "claude:fx-a"); err == nil || err.Error() != "no_selected_cases" {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildEpisodePromptProfiles(t *testing.T) {
	fixture := testFixtures()[0]
	clean, cleanSHA := BuildEpisodePrompt(fixture, "clean")
	if cleanSHA != "" || !strings.Contains(clean, "probe_b") || !strings.Contains(clean, `{"x":1}`) || strings.HasPrefix(clean, "CONFORMANCE_CONTEXT_PRESSURE") {
		t.Fatalf("clean prompt=%q sha=%q", clean, cleanSHA)
	}
	pressure, pressureSHA := BuildEpisodePrompt(fixture, "context-pressure")
	if len(pressureSHA) != 64 || !strings.HasPrefix(pressure, "CONFORMANCE_CONTEXT_PRESSURE_FIXED_BLOCK\n") || !strings.HasSuffix(pressure, clean) {
		t.Fatalf("pressure prompt malformed sha=%q", pressureSHA)
	}
	if len(pressure) != contextPressureBytes+1+len(clean) {
		t.Fatalf("pressure len=%d", len(pressure))
	}
	again, againSHA := BuildEpisodePrompt(testFixtures()[1], "context-pressure")
	if againSHA != pressureSHA || again == pressure {
		t.Fatalf("appendix digest must be fixture independent while the instruction differs")
	}
}

func TestRunLiveBenchmarkRejectsBadPreviousAndManifestErrors(t *testing.T) {
	deps := liveDeps(map[string]port.HostProbeRunner{}, testFixtures())
	request := baseRequest()
	request.Previous = &contract.BenchmarkReport{SchemaVersion: 1, Profile: "clean"}
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "unsupported_previous_report_schema:1" {
		t.Fatalf("err=%v", err)
	}
	request.Previous = &contract.BenchmarkReport{SchemaVersion: contract.ReportSchemaVersion, Profile: "context-pressure"}
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "invalid_previous_report_identity" {
		t.Fatalf("err=%v", err)
	}
	request.Previous = nil
	deps.LoadManifest = func([]contract.ToolDescriptor) ([]contract.Fixture, []contract.BaselineCase, error) {
		return nil, nil, context.DeadlineExceeded
	}
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err != context.DeadlineExceeded {
		t.Fatalf("err=%v", err)
	}
	deps = liveDeps(nil, testFixtures())
	request.Only = "codex:missing"
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "no_selected_cases" {
		t.Fatalf("err=%v", err)
	}
	request.Only = ""
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "unsupported_host:codex" {
		t.Fatalf("err=%v", err)
	}
}

func TestRunLiveBenchmarkCompletesEveryFixtureAndSupportsHost(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: evidenceByAttempt("codex")}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.Models = map[string]string{"codex": "  gpt-x "}
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil || !report.OK {
		t.Fatalf("report=%+v err=%v", report, err)
	}
	if report.RunID != "run-1" || report.CaseCount != 2 || report.SchemaVersion != contract.ReportSchemaVersion {
		t.Fatalf("header=%+v", report)
	}
	host := report.Hosts[0]
	if host.Status != contract.HostStatusSupported || !host.Evidence.LiveVerified || !host.Evidence.LiveAttempted || host.Evidence.StatusReason != "" {
		t.Fatalf("host=%+v", host)
	}
	if host.Version != "1.2.3" || host.RequestedModel != "gpt-x" || host.ObservedModel != "observed-model" || host.CompletedEpisodes != 2 || host.AttemptCount != 2 {
		t.Fatalf("host=%+v", host)
	}
	if host.Cases[0].FixtureID != "fx-a" || host.Cases[1].FixtureID != "fx-b" {
		t.Fatalf("cases not sorted by fixture: %+v", host.Cases)
	}
	if report.Counts.Attempts != 2 || report.Counts.Completed != 2 || report.Counts.ModelDenominator != 2 || report.Gate.Decision == contract.GateInconclusive {
		t.Fatalf("counts=%+v gate=%+v", report.Counts, report.Gate)
	}
	first := runner.requests[0]
	if first.FixtureID != "fx-b" || first.ProbeTool != "probe_b" || first.SourceTool != "tool_b" || first.ExpectedArgumentsJSON != `{"x":1}` ||
		first.Model != "gpt-x" || first.RunToken != "t" || runner.requests[1].RunToken != "tt" || first.HostVersion != "1.2.3" || first.Attempt != 1 {
		t.Fatalf("request=%+v", first)
	}
}

func TestRunLiveBenchmarkGeneratesRunIDAndProfileDigest(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: evidenceByAttempt("codex")}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.RunID = ""
	request.Profile = "context-pressure"
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if report.RunID != "20260102T030405.000000006Z" || len(report.ProfileSHA256) != 64 || report.Profile != "context-pressure" {
		t.Fatalf("report=%+v", report)
	}
	if report.Hosts[0].RequestedModel != "default" {
		t.Fatalf("model=%q", report.Hosts[0].RequestedModel)
	}
}

func TestRunLiveBenchmarkPreflightFailuresMapToHostStatus(t *testing.T) {
	cases := []struct {
		name      string
		preflight port.HostProbePreflight
		status    contract.HostStatus
		reason    string
	}{
		{"not installed", port.HostProbePreflight{Version: "v", Cause: "harness_environment", Code: "not_installed"}, contract.HostStatusUnavailable, "not_installed"},
		{"version probe", port.HostProbePreflight{Installed: true, Code: "version_probe_failed", Cause: "transport"}, contract.HostStatusUnavailable, "version_probe_failed"},
		{"mock extension", port.HostProbePreflight{Installed: true, Code: "mock_extension_invalid", Cause: "contract_input", EvidenceSource: "codex_preflight"}, contract.HostStatusUnsupported, "mock_extension_invalid"},
		{"other", port.HostProbePreflight{Installed: true, Code: "auth_missing", Cause: "bogus"}, contract.HostStatusNotRun, "auth_missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &scriptedRunner{preflight: tc.preflight, script: func(int, port.HostProbeRequest) port.HostProbeResult {
				t.Fatal("runner must not run after failed preflight")
				return port.HostProbeResult{}
			}}
			deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
			report, err := RunLiveBenchmark(context.Background(), baseRequest(), nil, deps)
			if err != nil {
				t.Fatal(err)
			}
			host := report.Hosts[0]
			if host.Status != tc.status || host.Evidence.StatusReason != tc.reason || host.Evidence.PreflightReady || host.Evidence.LiveAttempted {
				t.Fatalf("host=%+v", host)
			}
			if len(host.Cases) != 2 || host.CompletedEpisodes != 0 {
				t.Fatalf("cases=%+v", host.Cases)
			}
			for _, episode := range host.Cases {
				if episode.Status != contract.EpisodeIncomplete || episode.Attempt != 0 || episode.Diagnostics == nil || len(episode.FailureCauseEvidence) != 1 || episode.FailureCauseEvidence[0].Code != tc.preflight.Code {
					t.Fatalf("episode=%+v", episode)
				}
			}
			if report.OK || report.Gate.Decision != contract.GateInconclusive {
				t.Fatalf("report=%+v", report)
			}
		})
	}
}

func TestRunLiveBenchmarkRetriesIncompleteAttemptsUpToLimit(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: func(call int, request port.HostProbeRequest) port.HostProbeResult {
		if call < 3 {
			return port.HostProbeResult{Host: "codex", HostVersion: request.HostVersion, Profile: request.Profile, Attempt: request.Attempt, FixtureID: request.FixtureID,
				Cause: "harness_environment", Code: "spawn_failed", ObservedModel: "ghost", DurationMS: 9}
		}
		return validResult("codex", request, hex64("d"))
	}}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.Only = "codex:fx-a"
	request.MaxAttemptsPerCase = 3
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	host := report.Hosts[0]
	if len(runner.requests) != 3 || host.AttemptCount != 3 || host.CompletedEpisodes != 1 {
		t.Fatalf("requests=%d host=%+v", len(runner.requests), host)
	}
	for i, episode := range host.Cases {
		if episode.Attempt != i+1 {
			t.Fatalf("attempt numbering: %+v", host.Cases)
		}
	}
	if host.Cases[0].FailureCause != failurecausecontract.HarnessEnvironment || host.Cases[0].DurationMS != 9 || host.Cases[0].ObservedModel != "ghost" {
		t.Fatalf("incomplete episode=%+v", host.Cases[0])
	}
	if report.Counts.EnvironmentFailures != 2 || report.Counts.Attempts != 3 || report.Counts.Completed != 1 {
		t.Fatalf("counts=%+v", report.Counts)
	}
	if !contains(report.Warnings, "environment_transport_failure_rate_above_5_percent") {
		t.Fatalf("warnings=%v", report.Warnings)
	}
	if host.Status != contract.HostStatusSupported {
		t.Fatalf("status=%s", host.Status)
	}
}

func contains(values []string, want string) bool {
	return slices.Contains(values, want)
}

func TestRunLiveBenchmarkHostWithNoCompletedEpisodeIsUnavailable(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: func(_ int, request port.HostProbeRequest) port.HostProbeResult {
		return port.HostProbeResult{Completed: true, Host: "codex", Classification: contract.NoCall, CallCount: 0, EvidenceID: hex64("e")}
	}}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.Only = "codex:fx-a"
	request.MaxAttemptsPerCase = 2
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	host := report.Hosts[0]
	if host.Status != contract.HostStatusUnavailable || host.Evidence.StatusReason != "live_probe_incomplete" || !host.Evidence.LiveAttempted || host.Evidence.LiveVerified {
		t.Fatalf("host=%+v", host)
	}
	if report.Counts.NoCalls != 2 || report.Counts.EnvironmentFailures != 0 || report.OK {
		t.Fatalf("counts=%+v ok=%v", report.Counts, report.OK)
	}
}

func TestClassifyHostResultOutcomes(t *testing.T) {
	policy := benchmarkPolicy{classify: failurecausedomain.Classify}
	fixture := testFixtures()[1]
	request := port.HostProbeRequest{HostVersion: "1", Model: "m", FixtureID: fixture.ID, SchemaSHA256: fixture.SchemaSHA256, Profile: "clean", Attempt: 1}
	valid := func(mutate func(*port.HostProbeResult)) port.HostProbeResult {
		result := validResult("codex", request, hex64("a"))
		mutate(&result)
		return result
	}
	invalid := map[string]port.HostProbeResult{
		"unknown classification": valid(func(r *port.HostProbeResult) { r.Classification = "weird" }),
		"empty evidence":         valid(func(r *port.HostProbeResult) { r.EvidenceID = "" }),
		"malformed evidence":     valid(func(r *port.HostProbeResult) { r.EvidenceID = "zz" }),
		"multiple mismatch":      valid(func(r *port.HostProbeResult) { r.CallCount = 2 }),
		"multiple class single":  valid(func(r *port.HostProbeResult) { r.Classification = contract.MultipleCalls }),
		"bad arguments json":     valid(func(r *port.HostProbeResult) { r.CanonicalArgumentsJSON = "{" }),
		"bad diagnostics json":   valid(func(r *port.HostProbeResult) { r.DiagnosticsJSON = "{" }),
		"exact but not valid":    valid(func(r *port.HostProbeResult) { r.CanonicalValid = false }),
		"exact with diagnostics": valid(func(r *port.HostProbeResult) { r.DiagnosticsJSON = `[{"code":"c"}]` }),
		"drift but valid":        valid(func(r *port.HostProbeResult) { r.Classification = contract.UnknownKey }),
	}
	for name, result := range invalid {
		episode := policy.classifyHostResult(result, fixture)
		if episode.Status != contract.EpisodeIncomplete || episode.FailureCause != failurecausecontract.Transport || episode.FailureCauseEvidence[0].Code != "probe_result_invalid" || episode.FailureCauseEvidence[0].Source != "codex_runner" {
			t.Errorf("%s: episode=%+v", name, episode)
		}
	}
	noCall := policy.classifyHostResult(valid(func(r *port.HostProbeResult) { r.CallCount = 0 }), fixture)
	if noCall.Status != contract.EpisodeIncomplete || noCall.FailureCauseEvidence[0].Code != "no_call" {
		t.Fatalf("noCall=%+v", noCall)
	}
	multiple := policy.classifyHostResult(valid(func(r *port.HostProbeResult) { r.CallCount = 2; r.Classification = contract.MultipleCalls }), fixture)
	if multiple.Status != contract.EpisodeCompleted || multiple.Classification != contract.Classification(contract.MultipleCalls) {
		t.Fatalf("multiple=%+v", multiple)
	}

	exact := policy.classifyHostResult(valid(func(*port.HostProbeResult) {}), fixture)
	if exact.Status != contract.EpisodeCompleted || len(exact.FailureCauseEvidence) != 0 || exact.FailureCause != failurecausecontract.None || exact.DiagnosticSignature == "" {
		t.Fatalf("exact=%+v", exact)
	}

	driftModel := policy.classifyHostResult(valid(func(r *port.HostProbeResult) {
		r.Classification = contract.EnumMismatch
		r.CanonicalValid = false
		r.AdvertisedValid = false
		r.DiagnosticsJSON = `[{"path":"/b","code":"enum"},{"path":"/a","code":"enum"}]`
	}), fixture)
	if driftModel.Status != contract.EpisodeCompleted || driftModel.FailureCauseEvidence[0].Cause != failurecausecontract.Model ||
		driftModel.Diagnostics[0].Path != "/a" || driftModel.Diagnostics[1].Path != "/b" {
		t.Fatalf("driftModel=%+v", driftModel)
	}
	driftContract := policy.classifyHostResult(valid(func(r *port.HostProbeResult) {
		r.Classification = contract.MissingRequired
		r.CanonicalValid = false
		r.DiagnosticsJSON = `null`
	}), fixture)
	if driftContract.FailureCauseEvidence[0].Cause != failurecausecontract.ContractInput || driftContract.Diagnostics == nil || len(driftContract.Diagnostics) != 0 {
		t.Fatalf("driftContract=%+v", driftContract)
	}
}

func TestRunLiveBenchmarkDetectsDuplicateEvidence(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: func(_ int, request port.HostProbeRequest) port.HostProbeResult {
		return validResult("codex", request, hex64("f"))
	}}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.Only = "codex:fx-a"
	request.TargetCompleted = 10
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "duplicate_episode_evidence" {
		t.Fatalf("err=%v", err)
	}
}

func TestRunLiveBenchmarkRejectsRunnerEpisodeFailingValidation(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: func(_ int, request port.HostProbeRequest) port.HostProbeResult {
		result := validResult("codex", request, hex64("a"))
		result.ObservedModel = "" // classified completed but observed model is required evidence
		return result
	}}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	request.Only = "codex:fx-a"
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	episode := report.Hosts[0].Cases[0]
	if episode.Status != contract.EpisodeIncomplete || episode.FailureCauseEvidence[0].Code != "probe_result_invalid" || episode.FailureCauseEvidence[0].Source != "codex_runner" || report.Counts.TransportFailures != 1 {
		t.Fatalf("episode=%+v counts=%+v", episode, report.Counts)
	}
}

func TestRunLiveBenchmarkResumesCompletedEpisodesWithoutRerunning(t *testing.T) {
	runner := &scriptedRunner{preflight: readyPreflight(), script: evidenceByAttempt("codex")}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": runner}, testFixtures())
	request := baseRequest()
	first, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil || len(runner.requests) != 2 {
		t.Fatalf("first err=%v calls=%d", err, len(runner.requests))
	}

	resumeRunner := &scriptedRunner{preflight: readyPreflight(), script: func(int, port.HostProbeRequest) port.HostProbeResult {
		t.Fatal("completed episodes must not be rerun")
		return port.HostProbeResult{}
	}}
	deps = liveDeps(map[string]port.HostProbeRunner{"codex": resumeRunner}, testFixtures())
	request.Previous = &first
	second, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if second.Hosts[0].CompletedEpisodes != 2 || second.Hosts[0].Status != contract.HostStatusSupported || !second.Hosts[0].Evidence.LiveAttempted || second.Hosts[0].ObservedModel != "observed-model" {
		t.Fatalf("resumed host=%+v", second.Hosts[0])
	}

	// Tampered previous evidence must be refused rather than carried over.
	tampered := first
	tampered.Hosts = append([]contract.HostReport(nil), first.Hosts...)
	tampered.Hosts[0].Cases = append([]contract.EpisodeReport(nil), first.Hosts[0].Cases...)
	tampered.Hosts[0].Cases[0].DurationMS = 0
	request.Previous = &tampered
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "invalid_previous_episode_evidence" {
		t.Fatalf("err=%v", err)
	}

	// Two previous episodes sharing one evidence id are refused.
	duplicated := first
	duplicated.Hosts = append([]contract.HostReport(nil), first.Hosts...)
	cases := append([]contract.EpisodeReport(nil), first.Hosts[0].Cases...)
	cases[1].EvidenceID = cases[0].EvidenceID
	duplicated.Hosts[0].Cases = cases
	request.Previous = &duplicated
	if _, err := RunLiveBenchmark(context.Background(), request, nil, deps); err == nil || err.Error() != "duplicate_previous_episode_evidence" {
		t.Fatalf("err=%v", err)
	}
}

func TestRunLiveBenchmarkRunsOnlySelectedPairsForEachHost(t *testing.T) {
	codex := &scriptedRunner{preflight: readyPreflight(), script: evidenceByAttempt("codex")}
	claude := &scriptedRunner{preflight: port.HostProbePreflight{Installed: false, Code: "not_installed"}, script: evidenceByAttempt("claude")}
	deps := liveDeps(map[string]port.HostProbeRunner{"codex": codex, "claude": claude}, testFixtures())
	request := baseRequest()
	request.Hosts = []string{"codex", "claude"}
	request.Only = "codex:fx-b"
	report, err := RunLiveBenchmark(context.Background(), request, nil, deps)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Hosts) != 2 || report.Hosts[0].Host != "codex" || report.Hosts[1].Host != "claude" {
		t.Fatalf("hosts=%+v", report.Hosts)
	}
	if len(codex.requests) != 1 || codex.requests[0].FixtureID != "fx-b" || len(claude.requests) != 0 || len(report.Hosts[1].Cases) != 0 {
		t.Fatalf("codex=%d claude=%d", len(codex.requests), len(claude.requests))
	}
	if report.Hosts[1].Status != contract.HostStatusUnavailable {
		t.Fatalf("claude=%+v", report.Hosts[1])
	}
}

func TestCountReportSeparatesFailureKinds(t *testing.T) {
	completed := func(class string) contract.EpisodeReport {
		return contract.EpisodeReport{Status: contract.EpisodeCompleted, Classification: contract.Classification(class)}
	}
	incomplete := func(cause failurecausecontract.Cause, code string) contract.EpisodeReport {
		return contract.EpisodeReport{Status: contract.EpisodeIncomplete, FailureCause: cause,
			FailureCauseEvidence: []failurecausecontract.Evidence{{Cause: cause, Code: code}}}
	}
	report := contract.BenchmarkReport{Hosts: []contract.HostReport{{Cases: []contract.EpisodeReport{
		completed(contract.ExactValid),
		completed(contract.ValidButSemanticallyDifferent),
		completed(contract.EnumMismatch),
		incomplete(failurecausecontract.HarnessEnvironment, "x"),
		incomplete(failurecausecontract.Transport, "y"),
		incomplete(failurecausecontract.Unknown, "no_call"),
		incomplete(failurecausecontract.Unknown, "z"),
	}}, {Cases: []contract.EpisodeReport{completed(contract.UnknownKey)}}}}
	got := countReport(report)
	want := contract.BenchmarkCounts{Attempts: 8, Completed: 4, ModelDenominator: 4, NoCalls: 1, EnvironmentFailures: 1, TransportFailures: 1, ValidSemanticDifferences: 1, SchemaDriftObservations: 2}
	if got != want {
		t.Fatalf("got=%+v want=%+v", got, want)
	}
}

func TestIncompleteEpisodeNormalizesCauseAndSource(t *testing.T) {
	policy := benchmarkPolicy{classify: failurecausedomain.Classify}
	fixture := testFixtures()[0]
	episode := policy.incompleteEpisode("codex", "1", fixture, "clean", "m", 2, "nonsense", "code", "")
	evidence := episode.FailureCauseEvidence
	if episode.Status != contract.EpisodeIncomplete || episode.SchemaSHA256 != fixture.SchemaSHA256 || episode.Attempt != 2 || len(evidence) != 1 ||
		evidence[0].Cause != failurecausecontract.Unknown || evidence[0].Source != "tool_conformance" || evidence[0].Code != "code" {
		t.Fatalf("episode=%+v", episode)
	}
	kept := policy.incompleteEpisode("codex", "1", fixture, "clean", "m", 2, "model", "code", "src")
	if kept.FailureCauseEvidence[0].Cause != failurecausecontract.Model || kept.FailureCauseEvidence[0].Source != "src" {
		t.Fatalf("kept=%+v", kept)
	}
}
