package toolconformance_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	core "issueops/internal/adapter/toolconformance"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type fakeProbeRunner struct {
	host      string
	fixtures  map[string]core.Fixture
	responses map[string][]map[string]any
	failCode  string
	calls     map[string]int
	preflight *port.HostProbePreflight
	mutate    func(*port.HostProbeResult)
}

func (f *fakeProbeRunner) Name() string { return f.host }

func (f *fakeProbeRunner) Preflight(context.Context, port.HostProbeRequest) port.HostProbePreflight {
	if f.preflight != nil {
		return *f.preflight
	}
	return port.HostProbePreflight{Ready: true, Installed: true, Host: f.host, Version: f.host + "-1", RequestedModel: "default"}
}

func (f *fakeProbeRunner) Run(_ context.Context, request port.HostProbeRequest) port.HostProbeResult {
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	index := f.calls[request.FixtureID]
	f.calls[request.FixtureID]++
	if f.failCode != "" {
		return port.HostProbeResult{Host: f.host, HostVersion: request.HostVersion, RequestedModel: request.Model, ObservedModel: request.Model, FixtureID: request.FixtureID, Profile: request.Profile, Attempt: request.Attempt, Cause: "transport", Code: f.failCode, EvidenceSource: f.host + "_runner"}
	}
	arguments := f.fixtures[request.FixtureID].ExpectedArguments
	if values := f.responses[request.FixtureID]; index < len(values) {
		arguments = values[index]
	}
	encoded, _ := json.Marshal(arguments)
	rawSHA := sha256.Sum256(encoded)
	evidenceSHA := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", f.host, request.FixtureID, request.RunToken, request.Attempt)))
	classification := core.Classification(core.ExactValid)
	advertisedValid, canonicalValid := true, true
	diagnostics := []core.Diagnostic{}
	if _, exists := arguments["requireUnique"]; exists {
		classification = core.Classification(core.UnknownKey)
		canonicalValid = false
		diagnostics = []core.Diagnostic{{Path: "/requireUnique", Code: core.UnknownKey, Expected: "declared property", Actual: "boolean"}}
	}
	diagnosticsJSON, _ := json.Marshal(diagnostics)
	result := port.HostProbeResult{
		Completed:              true,
		Host:                   f.host,
		HostVersion:            request.HostVersion,
		RequestedModel:         request.Model,
		ObservedModel:          request.Model,
		FixtureID:              request.FixtureID,
		SchemaSHA256:           request.SchemaSHA256,
		Profile:                request.Profile,
		Attempt:                request.Attempt,
		DurationMS:             1,
		SessionStartObserved:   true,
		PreToolUseObserved:     false,
		AmbientToolCount:       1,
		CallCount:              1,
		ResponseSHA256:         fmt.Sprintf("%x", sha256.Sum256([]byte("response"))),
		ExitCode:               0,
		RawArgumentsSHA256:     fmt.Sprintf("%x", rawSHA),
		CanonicalArgumentsJSON: string(encoded),
		EvidenceID:             fmt.Sprintf("%x", evidenceSHA),
		Classification:         string(classification),
		AdvertisedValid:        advertisedValid,
		CanonicalValid:         canonicalValid,
		DiagnosticsJSON:        string(diagnosticsJSON),
	}
	if f.mutate != nil {
		f.mutate(&result)
	}
	return result
}

func TestLiveReportSeparatesInstalledMockAndLiveEvidence(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runner := &fakeProbeRunner{
		host: "omo", fixtures: fixtures, responses: map[string][]map[string]any{},
		preflight: &port.HostProbePreflight{
			Ready: true, Installed: true, MockExtensionVerified: true,
			Host: "omo", Version: "omo 5.0.0-0.beta.22", RequestedModel: "google/gemini-2.5-pro",
		},
	}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"omo"}, Models: map[string]string{"omo": "google/gemini-2.5-pro"}, Profile: "clean",
		Only: "omo:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "omo-live",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"omo": runner}, Token: func() string { return "token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	host := report.Hosts[0]
	if host.Status != issueopscontract.StatusSupported || !host.Evidence.Installed || !host.Evidence.PreflightReady ||
		!host.Evidence.MockExtensionVerified || !host.Evidence.LiveAttempted || !host.Evidence.LiveVerified || host.Evidence.StatusReason != "" {
		t.Fatalf("host evidence = %+v status=%q", host.Evidence, host.Status)
	}
	episode := host.Cases[0]
	if !episode.SessionStartObserved || episode.PreToolUseObserved || episode.ResponseSHA256 == "" || episode.ExitCode != 0 || episode.DurationMS != 1 {
		t.Fatalf("episode runtime evidence = %+v", episode)
	}
}

func TestLiveReportValidatesFreshCompletedEvidenceForEveryHost(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	for _, host := range []string{"codex", "claude", "omo"} {
		t.Run(host+" good", func(t *testing.T) {
			runner := &fakeProbeRunner{host: host, fixtures: fixtures, responses: map[string][]map[string]any{}}
			report, err := runSingleFreshEpisode(t, host, runner)
			if err != nil {
				t.Fatal(err)
			}
			got := report.Hosts[0]
			if got.Status != issueopscontract.StatusSupported || !got.Evidence.LiveVerified || got.CompletedEpisodes != 1 {
				t.Fatalf("host report = %+v", got)
			}
		})

		for _, invalid := range []struct {
			name   string
			mutate func(*port.HostProbeResult)
		}{
			{name: "host", mutate: func(result *port.HostProbeResult) { result.Host = "other" }},
			{name: "version", mutate: func(result *port.HostProbeResult) { result.HostVersion = "stale" }},
			{name: "requested model", mutate: func(result *port.HostProbeResult) { result.RequestedModel = "other-model" }},
			{name: "observed model", mutate: func(result *port.HostProbeResult) { result.ObservedModel = "" }},
			{name: "fixture", mutate: func(result *port.HostProbeResult) { result.FixtureID = "other" }},
			{name: "schema", mutate: func(result *port.HostProbeResult) { result.SchemaSHA256 = strings.Repeat("b", 64) }},
			{name: "profile", mutate: func(result *port.HostProbeResult) { result.Profile = "context-pressure" }},
			{name: "attempt", mutate: func(result *port.HostProbeResult) { result.Attempt++ }},
			{name: "duration", mutate: func(result *port.HostProbeResult) { result.DurationMS = 0 }},
			{name: "context", mutate: func(result *port.HostProbeResult) { result.SessionStartObserved = false }},
			{name: "unsupported pre tool claim", mutate: func(result *port.HostProbeResult) { result.PreToolUseObserved = true }},
			{name: "ambient count", mutate: func(result *port.HostProbeResult) { result.AmbientToolCount = 2 }},
			{name: "target count", mutate: func(result *port.HostProbeResult) { result.CallCount = 2 }},
			{name: "response digest", mutate: func(result *port.HostProbeResult) { result.ResponseSHA256 = "bad" }},
			{name: "exit", mutate: func(result *port.HostProbeResult) { result.ExitCode = 7 }},
			{name: "raw digest", mutate: func(result *port.HostProbeResult) { result.RawArgumentsSHA256 = "bad" }},
			{name: "evidence id", mutate: func(result *port.HostProbeResult) { result.EvidenceID = "bad" }},
			{name: "canonical arguments", mutate: func(result *port.HostProbeResult) { result.CanonicalArgumentsJSON = "null" }},
			{name: "diagnostics", mutate: func(result *port.HostProbeResult) {
				result.DiagnosticsJSON = `[{"path":"/x","code":"","expected":"","actual":""}]`
			}},
		} {
			t.Run(host+" bad "+invalid.name, func(t *testing.T) {
				runner := &fakeProbeRunner{host: host, fixtures: fixtures, responses: map[string][]map[string]any{}, mutate: invalid.mutate}
				report, err := runSingleFreshEpisode(t, host, runner)
				if err != nil {
					t.Fatal(err)
				}
				got := report.Hosts[0]
				if got.Status == issueopscontract.StatusSupported || got.Evidence.LiveVerified || got.CompletedEpisodes != 0 || len(got.Cases) != 1 || got.Cases[0].Status != core.EpisodeIncomplete {
					t.Fatalf("invalid fresh evidence was promoted: %+v", got)
				}
			})
		}
	}
}

func runSingleFreshEpisode(t *testing.T, host string, runner port.HostProbeRunner) (core.BenchmarkReport, error) {
	t.Helper()
	models := map[string]string{host: "model-a"}
	return core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{host}, Models: models, Profile: "clean", Only: host + ":empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: host + "-fresh",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{host: runner}, Token: func() string { return host + "-token" },
	})
}

func TestLiveReportKeepsInstalledOmoWithoutEpisodeNotRun(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runner := &fakeProbeRunner{
		host: "omo", fixtures: fixtures,
		preflight: &port.HostProbePreflight{
			Installed: true, MockExtensionVerified: true, Host: "omo", Version: "omo 5.0.0-0.beta.22",
			Cause: "harness_environment", Code: "explicit_model_required", EvidenceSource: "omo_preflight",
		},
	}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"omo"}, Profile: "clean", Only: "omo:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "omo-not-run",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"omo": runner}, Token: func() string { return "token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	host := report.Hosts[0]
	if host.Status != issueopscontract.StatusNotRun || !host.Evidence.Installed || host.Evidence.PreflightReady ||
		!host.Evidence.MockExtensionVerified || host.Evidence.LiveAttempted || host.Evidence.LiveVerified || host.Evidence.StatusReason != "explicit_model_required" {
		t.Fatalf("host evidence = %+v status=%q", host.Evidence, host.Status)
	}
	if runner.calls["empty_object"] != 0 {
		t.Fatalf("live episode calls = %d", runner.calls["empty_object"])
	}
}

func TestLiveReportMarksUnavailableExecutableWithoutClaimingSupport(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runner := &fakeProbeRunner{
		host: "omo", fixtures: fixtures,
		preflight: &port.HostProbePreflight{
			Host: "omo", MockExtensionVerified: true, Cause: "harness_environment", Code: "executable_not_found", EvidenceSource: "omo_preflight",
		},
	}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"omo"}, Profile: "clean", Only: "omo:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "omo-unavailable",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"omo": runner}, Token: func() string { return "token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	host := report.Hosts[0]
	if host.Status != issueopscontract.StatusUnavailable || host.Evidence.Installed || host.Evidence.LiveAttempted || host.Evidence.LiveVerified || host.Evidence.StatusReason != "executable_not_found" {
		t.Fatalf("host evidence = %+v status=%q", host.Evidence, host.Status)
	}
}

func TestLiveGateSixExactEpisodesDeferHardening(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runners := map[string]port.HostProbeRunner{}
	for _, host := range []string{"codex", "claude"} {
		runners[host] = &fakeProbeRunner{host: host, fixtures: fixtures, responses: map[string][]map[string]any{}}
	}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex", "claude"}, Profile: "clean", TargetCompleted: 1, MaxAttemptsPerCase: 3, HarnessBinary: "/harness", RunID: "exact",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: runners, Now: func() time.Time { return time.Unix(1, 0) }, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if !report.OK || report.Gate.Decision != core.GateDeferHardening || report.Counts.Completed != 6 || report.Counts.ModelDenominator != 6 {
		t.Fatalf("report=%+v", report)
	}
}

func TestLiveGateIncompleteEpisodeFailsClosed(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "probe_result_missing"}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "incomplete",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if report.OK || report.Gate.Decision != core.GateInconclusive || report.Counts.TransportFailures != 1 || report.Counts.ModelDenominator != 0 {
		t.Fatalf("report=%+v", report)
	}
}

func TestLiveGateResumeConfirmsOnlyRepeatedDiagnosticSignature(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	invalid := cloneArguments(fixtures["empty_object"].ExpectedArguments)
	invalid["requireUnique"] = true
	initialRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	initial, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "initial",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": initialRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if initial.Gate.Decision != core.GateNeedsReproduction {
		t.Fatalf("initial gate=%s", initial.Gate.Decision)
	}
	reproductionRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	confirmed, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 10, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "confirmed", Previous: &initial,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": reproductionRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	if confirmed.Gate.Decision != core.GateAuthorizeHardening || confirmed.Gate.ConfirmedCount != 2 || confirmed.Counts.Completed != 10 {
		t.Fatalf("confirmed=%+v", confirmed)
	}
}

func TestLiveGateResumeRejectsStaleSchemaEvidence(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	invalid := cloneArguments(fixtures["empty_object"].ExpectedArguments)
	invalid["requireUnique"] = true
	initialRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	initial, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "initial",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": initialRunner}, Token: func() string { return "token" }})
	if err != nil {
		t.Fatal(err)
	}
	initial.Hosts[0].Cases[0].SchemaSHA256 = "stale"

	reproductionRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{"empty_object": {invalid}}}
	_, err = core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Profile: "clean", Only: "codex:empty_object", TargetCompleted: 10, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "reproduction", Previous: &initial,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": reproductionRunner}, Token: func() string { return "token" }})
	if err == nil || err.Error() != "invalid_previous_episode_evidence" {
		t.Fatalf("err=%v", err)
	}
	if got := reproductionRunner.calls["empty_object"]; got != 0 {
		t.Fatalf("stale evidence triggered fresh calls=%d", got)
	}
}

func TestLiveGateReusesOnlyCertifiedSchemaV2Episode(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	initialRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	initial, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "initial",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": initialRunner}, Token: func() string { return "token-a" }})
	if err != nil {
		t.Fatal(err)
	}

	resumeRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	resumed, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "resumed", Previous: &initial,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": resumeRunner}, Token: func() string { return "token-b" }})
	if err != nil {
		t.Fatal(err)
	}
	if resumeRunner.calls["empty_object"] != 0 {
		t.Fatalf("fresh calls = %d, want reused evidence", resumeRunner.calls["empty_object"])
	}
	host := resumed.Hosts[0]
	if host.Status != issueopscontract.StatusSupported || !host.Evidence.LiveAttempted || !host.Evidence.LiveVerified || host.CompletedEpisodes != 1 {
		t.Fatalf("resumed host = %+v", host)
	}
}

func TestLiveGateRejectsUnselectedCompletedPreviousEpisode(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previousRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	previous, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "all-completed",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"codex": previousRunner}, Token: func() string { return "previous-token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if previous.Hosts[0].CompletedEpisodes != len(fixtures) {
		t.Fatalf("completed episodes = %d, want %d", previous.Hosts[0].CompletedEpisodes, len(fixtures))
	}

	resumeRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	_, err = resumeCertifiedReport(t, previous, resumeRunner)
	if err == nil || err.Error() != "invalid_previous_episode_selection" {
		t.Fatalf("err = %v", err)
	}
	if resumeRunner.calls["empty_object"] != 0 {
		t.Fatalf("unselected completed evidence triggered %d fresh calls", resumeRunner.calls["empty_object"])
	}
}

func TestLiveGateRejectsUnselectedIncompletePreviousEpisode(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previousRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "host_process_failed"}
	previous, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "all-incomplete",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"codex": previousRunner}, Token: func() string { return "previous-token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if previous.Hosts[0].CompletedEpisodes != 0 || previous.Hosts[0].AttemptCount != len(fixtures) {
		t.Fatalf("previous host = %+v", previous.Hosts[0])
	}

	resumeRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	_, err = resumeCertifiedReport(t, previous, resumeRunner)
	if err == nil || err.Error() != "invalid_previous_episode_selection" {
		t.Fatalf("err = %v", err)
	}
	if resumeRunner.calls["empty_object"] != 0 {
		t.Fatalf("unselected incomplete evidence triggered %d fresh calls", resumeRunner.calls["empty_object"])
	}
}

func TestLiveGateRejectsPreviousOnlyHostEpisodes(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	tests := []struct {
		name              string
		claudeFailureCode string
		wantStatus        string
	}{
		{name: "completed", wantStatus: "completed"},
		{name: "incomplete", claudeFailureCode: "host_process_failed", wantStatus: "incomplete"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
				Hosts: []string{"codex", "claude"}, Models: map[string]string{"codex": "model-a", "claude": "model-b"}, Profile: "clean",
				TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "two-hosts",
			}, catalogDescriptors(), core.LiveBenchmarkDependencies{
				Runners: map[string]port.HostProbeRunner{
					"codex":  &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}},
					"claude": &fakeProbeRunner{host: "claude", fixtures: fixtures, responses: map[string][]map[string]any{}, failCode: test.claudeFailureCode},
				},
				Token: func() string { return "previous-token" },
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(previous.Hosts) != 2 || len(previous.Hosts[1].Cases) == 0 || string(previous.Hosts[1].Cases[0].Status) != test.wantStatus {
				t.Fatalf("previous claude host = %+v", previous.Hosts)
			}

			resumeRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
			_, err = core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
				Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean",
				TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "one-host", Previous: &previous,
			}, catalogDescriptors(), core.LiveBenchmarkDependencies{
				Runners: map[string]port.HostProbeRunner{"codex": resumeRunner}, Token: func() string { return "new-token" },
			})
			if err == nil || err.Error() != "invalid_previous_episode_selection" {
				t.Fatalf("err = %v", err)
			}
			if len(resumeRunner.calls) != 0 {
				t.Fatalf("previous-only host triggered fresh calls: %+v", resumeRunner.calls)
			}
		})
	}
}

func TestLiveGateRejectsInvalidPreviousHostRows(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	baseline := certifiedPreviousReport(t, fixtures)
	tests := []struct {
		name   string
		mutate func(*core.BenchmarkReport)
	}{
		{name: "duplicate host", mutate: func(report *core.BenchmarkReport) {
			report.Hosts = append(report.Hosts, report.Hosts[0])
		}},
		{name: "missing host identity", mutate: func(report *core.BenchmarkReport) {
			report.Hosts = append(report.Hosts, core.HostReport{})
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := cloneBenchmarkReport(t, baseline)
			test.mutate(&previous)
			runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
			_, err := resumeCertifiedReport(t, previous, runner)
			if err == nil || err.Error() != "invalid_previous_report_identity" {
				t.Fatalf("err = %v", err)
			}
			if len(runner.calls) != 0 {
				t.Fatalf("invalid host rows triggered fresh calls: %+v", runner.calls)
			}
		})
	}
}

func TestLiveGateResumesSelectedZeroCompletedReport(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previousRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "host_process_failed"}
	previous, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "selected-incomplete",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"codex": previousRunner}, Token: func() string { return "previous-token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if previous.Hosts[0].CompletedEpisodes != 0 || previous.Hosts[0].AttemptCount != 1 {
		t.Fatalf("previous host = %+v", previous.Hosts[0])
	}

	resumeRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	resumed, err := resumeCertifiedReport(t, previous, resumeRunner)
	if err != nil {
		t.Fatal(err)
	}
	if resumeRunner.calls["empty_object"] != 1 {
		t.Fatalf("fresh calls = %d, want 1", resumeRunner.calls["empty_object"])
	}
	host := resumed.Hosts[0]
	if host.Status != issueopscontract.StatusSupported || !host.Evidence.LiveAttempted || !host.Evidence.LiveVerified || host.CompletedEpisodes != 1 {
		t.Fatalf("resumed host = %+v", host)
	}
}

func TestLiveGateRejectsSchemaV1ResumeWithoutAdditiveMigration(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previous := certifiedPreviousReport(t, fixtures)
	previous.SchemaVersion = 1
	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	_, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "schema-1", Previous: &previous,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "new-token" }})
	if err == nil || err.Error() != "unsupported_previous_report_schema:1" {
		t.Fatalf("err = %v", err)
	}
	if runner.calls["empty_object"] != 0 {
		t.Fatalf("schema-v1 report triggered %d fresh calls", runner.calls["empty_object"])
	}
}

func TestLiveGateRejectsLegacyShapedAndIdentityDriftedSchemaV2Episodes(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	baseline := certifiedPreviousReport(t, fixtures)
	tests := []struct {
		name   string
		mutate func(*core.BenchmarkReport)
	}{
		{name: "legacy shaped runtime proof", mutate: func(report *core.BenchmarkReport) {
			episode := &report.Hosts[0].Cases[0]
			episode.ObservedModel = ""
			episode.DurationMS = 0
			episode.SessionStartObserved = false
			episode.AmbientToolCount = 0
			episode.ResponseSHA256 = ""
		}},
		{name: "host", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].Host = "claude" }},
		{name: "host version", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].HostVersion = "old" }},
		{name: "profile", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].Profile = "context-pressure" }},
		{name: "requested model", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].RequestedModel = "model-b" }},
		{name: "observed model", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].ObservedModel = "model-b" }},
		{name: "schema digest", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].SchemaSHA256 = strings.Repeat("b", 64) }},
		{name: "response digest", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].ResponseSHA256 = "bad" }},
		{name: "exit", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].ExitCode = 7 }},
		{name: "duration", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].DurationMS = 0 }},
		{name: "context", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].SessionStartObserved = false }},
		{name: "ambient count", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].AmbientToolCount = 2 }},
		{name: "one call", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].CallCount = 2 }},
		{name: "evidence id", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].EvidenceID = "bad" }},
		{name: "raw digest", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].RawArgumentsSHA256 = "bad" }},
		{name: "diagnostic cause", mutate: func(report *core.BenchmarkReport) { report.Hosts[0].Cases[0].FailureCause = "model" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := cloneBenchmarkReport(t, baseline)
			test.mutate(&previous)
			runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
			_, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
				Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
				TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "malformed", Previous: &previous,
			}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "new-token" }})
			if err == nil || err.Error() != "invalid_previous_episode_evidence" {
				t.Fatalf("err = %v", err)
			}
			if runner.calls["empty_object"] != 0 {
				t.Fatalf("invalid evidence triggered %d fresh calls", runner.calls["empty_object"])
			}
		})
	}
}

func TestLiveGateRejectsDuplicateEpisodeIdentityWithDifferentEvidenceIDs(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previous := certifiedPreviousReport(t, fixtures)
	copy := previous.Hosts[0].Cases[0]
	copy.EvidenceID = fmt.Sprintf("%x", sha256.Sum256([]byte("different-evidence")))
	previous.Hosts[0].Cases = append(previous.Hosts[0].Cases, copy)
	previous.Hosts[0].AttemptCount = 2
	previous.Hosts[0].CompletedEpisodes = 2

	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	_, err := resumeCertifiedReport(t, previous, runner)
	if err == nil {
		t.Fatal("duplicate (host, fixture, attempt) identity was accepted")
	}
	if runner.calls["empty_object"] != 0 {
		t.Fatalf("duplicate identity triggered %d fresh calls", runner.calls["empty_object"])
	}
}

func TestLiveGateRejectsInconsistentPreviousHostSummary(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	baseline := certifiedPreviousReport(t, fixtures)
	tests := []struct {
		name   string
		mutate func(*core.HostReport)
	}{
		{name: "attempt count", mutate: func(host *core.HostReport) { host.AttemptCount = 0 }},
		{name: "completed count", mutate: func(host *core.HostReport) { host.CompletedEpisodes = 0 }},
		{name: "status", mutate: func(host *core.HostReport) { host.Status = issueopscontract.StatusNotRun }},
		{name: "live attempted", mutate: func(host *core.HostReport) { host.Evidence.LiveAttempted = false }},
		{name: "live verified", mutate: func(host *core.HostReport) { host.Evidence.LiveVerified = false }},
		{name: "status reason", mutate: func(host *core.HostReport) { host.Evidence.StatusReason = "stale" }},
		{name: "observed model", mutate: func(host *core.HostReport) { host.ObservedModel = "other-model" }},
		{name: "episode observed model", mutate: func(host *core.HostReport) { host.Cases[0].ObservedModel = "other-model" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := cloneBenchmarkReport(t, baseline)
			test.mutate(&previous.Hosts[0])
			runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
			_, err := resumeCertifiedReport(t, previous, runner)
			if err == nil {
				t.Fatal("inconsistent previous host summary was accepted")
			}
			if runner.calls["empty_object"] != 0 {
				t.Fatalf("invalid summary triggered %d fresh calls", runner.calls["empty_object"])
			}
		})
	}
}

func TestLiveGateRejectsInconsistentIncompletePreviousHostSummary(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	failedRunner := &fakeProbeRunner{host: "codex", fixtures: fixtures, failCode: "host_process_failed"}
	baseline, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "failed",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"codex": failedRunner}, Token: func() string { return "failed-token" },
	})
	if err != nil {
		t.Fatal(err)
	}
	if baseline.Hosts[0].CompletedEpisodes != 0 || baseline.Hosts[0].Status != issueopscontract.StatusUnavailable {
		t.Fatalf("failed baseline = %+v", baseline.Hosts[0])
	}
	tests := []struct {
		name   string
		mutate func(*core.HostReport)
	}{
		{name: "installed", mutate: func(host *core.HostReport) { host.Evidence.Installed = false }},
		{name: "preflight ready", mutate: func(host *core.HostReport) { host.Evidence.PreflightReady = false }},
		{name: "live attempted", mutate: func(host *core.HostReport) { host.Evidence.LiveAttempted = false }},
		{name: "live verified", mutate: func(host *core.HostReport) { host.Evidence.LiveVerified = true }},
		{name: "status", mutate: func(host *core.HostReport) { host.Status = issueopscontract.StatusSupported }},
		{name: "status reason", mutate: func(host *core.HostReport) { host.Evidence.StatusReason = "" }},
		{name: "observed model", mutate: func(host *core.HostReport) { host.ObservedModel = "other-model" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			previous := cloneBenchmarkReport(t, baseline)
			test.mutate(&previous.Hosts[0])
			runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
			_, err := resumeCertifiedReport(t, previous, runner)
			if err == nil {
				t.Fatal("inconsistent incomplete previous host summary was accepted")
			}
			if runner.calls["empty_object"] != 0 {
				t.Fatalf("invalid incomplete summary triggered %d fresh calls", runner.calls["empty_object"])
			}
		})
	}
}

func TestLiveGateRejectsExtraCompletedEpisodesBeyondCurrentTarget(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	previous := certifiedPreviousReport(t, fixtures)
	extra := previous.Hosts[0].Cases[0]
	extra.Attempt = 2
	extra.EvidenceID = fmt.Sprintf("%x", sha256.Sum256([]byte("extra-completed")))
	previous.Hosts[0].Cases = append(previous.Hosts[0].Cases, extra)
	previous.Hosts[0].AttemptCount = 2
	previous.Hosts[0].CompletedEpisodes = 2

	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	_, err := resumeCertifiedReport(t, previous, runner)
	if err == nil {
		t.Fatal("extra completed evidence beyond the requested target was accepted")
	}
	if runner.calls["empty_object"] != 0 {
		t.Fatalf("extra completed evidence triggered %d fresh calls", runner.calls["empty_object"])
	}
}

func resumeCertifiedReport(t *testing.T, previous core.BenchmarkReport, runner port.HostProbeRunner) (core.BenchmarkReport, error) {
	t.Helper()
	return core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "resumed-invalid", Previous: &previous,
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{
		Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "new-token" },
	})
}

func certifiedPreviousReport(t *testing.T, fixtures map[string]core.Fixture) core.BenchmarkReport {
	t.Helper()
	runner := &fakeProbeRunner{host: "codex", fixtures: fixtures, responses: map[string][]map[string]any{}}
	report, err := core.RunLiveBenchmark(context.Background(), core.LiveBenchmarkRequest{
		Hosts: []string{"codex"}, Models: map[string]string{"codex": "model-a"}, Profile: "clean", Only: "codex:empty_object",
		TargetCompleted: 1, MaxAttemptsPerCase: 1, HarnessBinary: "/harness", RunID: "certified",
	}, catalogDescriptors(), core.LiveBenchmarkDependencies{Runners: map[string]port.HostProbeRunner{"codex": runner}, Token: func() string { return "certified-token" }})
	if err != nil {
		t.Fatal(err)
	}
	return report
}

func cloneBenchmarkReport(t *testing.T, report core.BenchmarkReport) core.BenchmarkReport {
	t.Helper()
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var cloned core.BenchmarkReport
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestContextPressureProfileIsFixedSizeAndHash(t *testing.T) {
	fixtures := benchmarkFixtures(t)
	promptA, hashA := core.BuildEpisodePrompt(fixtures["empty_object"], "context-pressure")
	promptB, hashB := core.BuildEpisodePrompt(fixtures["empty_object"], "context-pressure")
	if promptA != promptB || hashA == "" || hashA != hashB {
		t.Fatalf("context profile is not deterministic")
	}
	clean, cleanHash := core.BuildEpisodePrompt(fixtures["empty_object"], "clean")
	if cleanHash != "" || len(promptA)-len(clean) < 32<<10 {
		t.Fatalf("context bytes=%d hash=%q", len(promptA)-len(clean), cleanHash)
	}
}

func benchmarkFixtures(t *testing.T) map[string]core.Fixture {
	t.Helper()
	fixtures, _, err := core.LoadManifest(catalogDescriptors())
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]core.Fixture{}
	for _, fixture := range fixtures {
		out[fixture.ID] = fixture
	}
	return out
}
