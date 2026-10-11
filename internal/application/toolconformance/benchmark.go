package toolconformance

import (
	fixturecontract "issueops/internal/contract/toolconformance"

	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	failurecausecontract "issueops/internal/contract/failurecause"
	toolconformancedomain "issueops/internal/domain/toolconformance"
	"issueops/internal/port"
)

const contextPressureBytes = 32 << 10

type LiveBenchmarkRequest struct {
	Hosts              []string
	Models             map[string]string
	Profile            string
	Only               string
	TargetCompleted    int
	MaxAttemptsPerCase int
	HarnessBinary      string
	RunID              string
	Previous           *fixturecontract.BenchmarkReport
}

type LiveBenchmarkDependencies struct {
	Runners      map[string]port.HostProbeRunner
	Now          func() time.Time
	Token        func() string
	LoadManifest func([]fixturecontract.ToolDescriptor) ([]fixturecontract.Fixture, []fixturecontract.BaselineCase, error)
	Classify     func(bool, []failurecausecontract.Evidence) failurecausecontract.Result
}

func RunLiveBenchmark(ctx context.Context, request LiveBenchmarkRequest, descriptors []fixturecontract.ToolDescriptor, deps LiveBenchmarkDependencies) (fixturecontract.BenchmarkReport, error) {
	if err := validateLiveRequest(request); err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	if err := validatePreviousReportIdentity(request); err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	fixtures, _, err := deps.LoadManifest(descriptors)
	if err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	selected, err := selectFixturePairs(request.Hosts, fixtures, request.Only)
	if err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	if err := toolconformancedomain.ValidatePreviousSelection(request.Previous, selected); err != nil {
		return fixturecontract.BenchmarkReport{}, err
	}
	report := fixturecontract.BenchmarkReport{
		OK:            true,
		SchemaVersion: fixturecontract.ReportSchemaVersion,
		RunID:         request.RunID,
		Profile:       request.Profile,
		CaseCount:     len(selected),
		Hosts:         []fixturecontract.HostReport{},
		Warnings:      []string{},
	}
	_, report.ProfileSHA256 = BuildEpisodePrompt(selected[0].Fixture, request.Profile)
	if report.RunID == "" {
		report.RunID = deps.Now().UTC().Format("20060102T150405.000000000Z")
	}
	run := liveRun{
		ctx: ctx, request: request, deps: deps,
		policy:          benchmarkPolicy{classify: deps.Classify},
		fixtures:        fixtures,
		selected:        selected,
		seenEvidenceIDs: map[string]bool{},
	}
	for _, host := range request.Hosts {
		hostReport, err := run.runHost(host)
		if err != nil {
			return fixturecontract.BenchmarkReport{}, err
		}
		report.Hosts = append(report.Hosts, hostReport)
	}
	finalizeLiveReport(&report, selected, request.TargetCompleted)
	return report, nil
}

func validatePreviousReportIdentity(request LiveBenchmarkRequest) error {
	if request.Previous == nil {
		return nil
	}
	if request.Previous.SchemaVersion != fixturecontract.ReportSchemaVersion {
		return fmt.Errorf("unsupported_previous_report_schema:%d", request.Previous.SchemaVersion)
	}
	if request.Previous.Profile != request.Profile {
		return fmt.Errorf("invalid_previous_report_identity")
	}
	return nil
}

func finalizeLiveReport(report *fixturecontract.BenchmarkReport, selected []toolconformancedomain.SelectedFixturePair, targetCompleted int) {
	report.Counts = countReport(*report)
	gatePairs := make([]toolconformancedomain.SelectedPair, 0, len(selected))
	for _, pair := range selected {
		gatePairs = append(gatePairs, toolconformancedomain.SelectedPair{Host: pair.Host, FixtureID: pair.Fixture.ID})
	}
	report.Gate = toolconformancedomain.DecideGate(*report, gatePairs, targetCompleted)
	if report.Gate.Decision == fixturecontract.GateInconclusive {
		report.OK = false
	}
	if report.Counts.Attempts > 0 && float64(report.Counts.EnvironmentFailures+report.Counts.TransportFailures)/float64(report.Counts.Attempts) > 0.05 {
		report.Warnings = append(report.Warnings, "environment_transport_failure_rate_above_5_percent")
	}
}

// liveRun carries the per-benchmark state shared by every host run.
type liveRun struct {
	ctx             context.Context
	request         LiveBenchmarkRequest
	deps            LiveBenchmarkDependencies
	policy          benchmarkPolicy
	fixtures        []fixturecontract.Fixture
	selected        []toolconformancedomain.SelectedFixturePair
	seenEvidenceIDs map[string]bool
}

func (r *liveRun) runHost(host string) (fixturecontract.HostReport, error) {
	runner := r.deps.Runners[host]
	if runner == nil {
		return fixturecontract.HostReport{}, fmt.Errorf("unsupported_host:%s", host)
	}
	hostReport := fixturecontract.HostReport{
		Status: fixturecontract.HostStatusNotRun,
		Host:   host, RequestedModel: modelForHost(r.request.Models, host), Cases: []fixturecontract.EpisodeReport{},
	}
	preflight := runner.Preflight(r.ctx, port.HostProbeRequest{HarnessBinary: r.request.HarnessBinary, Model: hostReport.RequestedModel})
	hostReport.Version = preflight.Version
	hostReport.Evidence.Installed = preflight.Installed
	hostReport.Evidence.PreflightReady = preflight.Ready
	hostReport.Evidence.MockExtensionVerified = preflight.MockExtensionVerified
	if preflight.ObservedModel != "" {
		hostReport.ObservedModel = preflight.ObservedModel
	}
	if preflight.Ready {
		if err := r.resumePreviousEpisodes(&hostReport, host, preflight); err != nil {
			return fixturecontract.HostReport{}, err
		}
	} else {
		applyPreflightFailure(&hostReport, preflight)
	}
	for _, pair := range r.selected {
		if pair.Host != host {
			continue
		}
		if !preflight.Ready {
			episode := r.policy.incompleteEpisode(host, preflight.Version, pair.Fixture, r.request.Profile, hostReport.RequestedModel, 0, preflight.Cause, preflight.Code, preflight.EvidenceSource)
			hostReport.Cases = append(hostReport.Cases, episode)
			continue
		}
		if err := r.runFixtureAttempts(&hostReport, runner, host, preflight, pair.Fixture); err != nil {
			return fixturecontract.HostReport{}, err
		}
	}
	sortEpisodes(hostReport.Cases)
	hostReport.AttemptCount = len(hostReport.Cases)
	hostReport.CompletedEpisodes = toolconformancedomain.CountCompleted(hostReport.Cases)
	applyHostStatus(&hostReport)
	return hostReport, nil
}

func applyPreflightFailure(hostReport *fixturecontract.HostReport, preflight port.HostProbePreflight) {
	hostReport.Evidence.StatusReason = preflight.Code
	if !preflight.Installed || preflight.Code == "version_probe_failed" {
		hostReport.Status = fixturecontract.HostStatusUnavailable
	} else if preflight.Code == "mock_extension_invalid" {
		hostReport.Status = fixturecontract.HostStatusUnsupported
	}
}

func applyHostStatus(hostReport *fixturecontract.HostReport) {
	if hostReport.Evidence.PreflightReady && hostReport.CompletedEpisodes > 0 {
		hostReport.Status = fixturecontract.HostStatusSupported
		hostReport.Evidence.LiveVerified = true
		hostReport.Evidence.StatusReason = ""
	} else if hostReport.Evidence.LiveAttempted {
		hostReport.Status = fixturecontract.HostStatusUnavailable
		hostReport.Evidence.StatusReason = "live_probe_incomplete"
	}
}

func sortEpisodes(cases []fixturecontract.EpisodeReport) {
	sort.SliceStable(cases, func(i, j int) bool {
		if cases[i].FixtureID != cases[j].FixtureID {
			return cases[i].FixtureID < cases[j].FixtureID
		}
		return cases[i].Attempt < cases[j].Attempt
	})
}

func (r *liveRun) resumePreviousEpisodes(hostReport *fixturecontract.HostReport, host string, preflight port.HostProbePreflight) error {
	previousHost, err := toolconformancedomain.ResumeHostReport(r.request.Previous, host, hostReport.RequestedModel, preflight.Version, r.request.Profile, r.fixtures, r.selected, r.request.TargetCompleted)
	if err != nil {
		return err
	}
	if previousHost == nil {
		return nil
	}
	for _, episode := range previousHost.Cases {
		fixture, selectedPair := toolconformancedomain.SelectedFixtureForPair(r.selected, host, episode.FixtureID)
		if !selectedPair || episode.Status != fixturecontract.EpisodeCompleted {
			continue
		}
		expectation := r.expectation(host, preflight.Version, hostReport.RequestedModel, fixture, episode.Attempt)
		if !r.policy.validCompletedEpisode(episode, expectation) {
			return fmt.Errorf("invalid_previous_episode_evidence")
		}
		if r.seenEvidenceIDs[episode.EvidenceID] {
			return fmt.Errorf("duplicate_previous_episode_evidence")
		}
		r.seenEvidenceIDs[episode.EvidenceID] = true
		hostReport.Cases = append(hostReport.Cases, episode)
		hostReport.ObservedModel = episode.ObservedModel
	}
	hostReport.Evidence.LiveAttempted = len(hostReport.Cases) > 0
	return nil
}

func (r *liveRun) expectation(host, hostVersion, requestedModel string, fixture fixturecontract.Fixture, attempt int) toolconformancedomain.CompletedEpisodeExpectation {
	return toolconformancedomain.CompletedEpisodeExpectation{
		Host: host, HostVersion: hostVersion, RequestedModel: requestedModel,
		Profile: r.request.Profile, Fixture: fixture, Attempt: attempt,
	}
}

func (r *liveRun) runFixtureAttempts(hostReport *fixturecontract.HostReport, runner port.HostProbeRunner, host string, preflight port.HostProbePreflight, fixture fixturecontract.Fixture) error {
	completed := toolconformancedomain.CompletedForFixture(hostReport.Cases, fixture.ID)
	newAttemptLimit := (r.request.TargetCompleted - completed) * r.request.MaxAttemptsPerCase
	for newAttempts := 0; completed < r.request.TargetCompleted && newAttempts < newAttemptLimit; newAttempts++ {
		hostReport.Evidence.LiveAttempted = true
		attempt := toolconformancedomain.AttemptsForFixture(hostReport.Cases, fixture.ID) + 1
		prompt, _ := BuildEpisodePrompt(fixture, r.request.Profile)
		runResult := runner.Run(r.ctx, port.HostProbeRequest{
			HarnessBinary:         r.request.HarnessBinary,
			HostVersion:           preflight.Version,
			FixtureID:             fixture.ID,
			ProbeTool:             fixture.ProbeTool,
			SourceTool:            fixture.SourceTool,
			SchemaSHA256:          fixture.SchemaSHA256,
			ExpectedArgumentsJSON: mustJSON(fixture.ExpectedArguments),
			Prompt:                prompt,
			Model:                 hostReport.RequestedModel,
			Profile:               r.request.Profile,
			Attempt:               attempt,
			RunToken:              r.deps.Token(),
		})
		episode := r.policy.classifyHostResult(runResult, fixture)
		expectation := r.expectation(host, preflight.Version, hostReport.RequestedModel, fixture, attempt)
		if episode.Status == fixturecontract.EpisodeCompleted && !r.policy.validCompletedEpisode(episode, expectation) {
			episode = r.policy.incompleteEpisode(host, preflight.Version, fixture, r.request.Profile, hostReport.RequestedModel, attempt, "transport", "probe_result_invalid", host+"_runner")
		}
		if episode.Status == fixturecontract.EpisodeCompleted {
			if r.seenEvidenceIDs[episode.EvidenceID] {
				return fmt.Errorf("duplicate_episode_evidence")
			}
			r.seenEvidenceIDs[episode.EvidenceID] = true
		}
		if episode.ObservedModel != "" {
			hostReport.ObservedModel = episode.ObservedModel
		}
		hostReport.Cases = append(hostReport.Cases, episode)
		if episode.Status == "completed" {
			completed++
		}
	}
	return nil
}

type benchmarkPolicy struct {
	classify func(bool, []failurecausecontract.Evidence) failurecausecontract.Result
}

func selectFixturePairs(hosts []string, fixtures []fixturecontract.Fixture, only string) ([]toolconformancedomain.SelectedFixturePair, error) {
	pairs := []toolconformancedomain.SelectedFixturePair{}
	onlyHost, onlyFixture := "", ""
	if only != "" {
		parts := strings.Split(only, ":")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid_only:%s", only)
		}
		onlyHost, onlyFixture = parts[0], parts[1]
	}
	for _, host := range hosts {
		for _, fixture := range fixtures {
			if only != "" && (host != onlyHost || fixture.ID != onlyFixture) {
				continue
			}
			pairs = append(pairs, toolconformancedomain.SelectedFixturePair{Host: host, Fixture: fixture})
		}
	}
	if len(pairs) == 0 {
		return nil, fmt.Errorf("no_selected_cases")
	}
	return pairs, nil
}

func validateLiveRequest(request LiveBenchmarkRequest) error {
	if len(request.Hosts) == 0 {
		return fmt.Errorf("hosts_required")
	}
	if request.Profile != "clean" && request.Profile != "context-pressure" {
		return fmt.Errorf("invalid_profile:%s", request.Profile)
	}
	if request.TargetCompleted != 1 && request.TargetCompleted != 10 && request.TargetCompleted != 20 {
		return fmt.Errorf("invalid_target_completed:%d", request.TargetCompleted)
	}
	if request.MaxAttemptsPerCase < 1 || request.MaxAttemptsPerCase > 3 {
		return fmt.Errorf("invalid_max_attempts_per_case:%d", request.MaxAttemptsPerCase)
	}
	if request.TargetCompleted > 1 && request.Only == "" {
		return fmt.Errorf("only_required_for_reproduction")
	}
	if request.HarnessBinary == "" {
		return fmt.Errorf("harness_binary_required")
	}
	return nil
}

func BuildEpisodePrompt(fixture fixturecontract.Fixture, profile string) (string, string) {
	instruction := fmt.Sprintf("Call the only allowed MCP tool %s exactly once with these exact JSON arguments. Do not add, remove, rename, or coerce any field. After the tool result, output only captured.\nArguments:\n%s", fixture.ProbeTool, mustJSON(fixture.ExpectedArguments))
	if profile != "context-pressure" {
		return instruction, ""
	}
	unit := "CONFORMANCE_CONTEXT_PRESSURE_FIXED_BLOCK\n"
	appendix := strings.Repeat(unit, contextPressureBytes/len(unit)+1)[:contextPressureBytes]
	sum := sha256.Sum256([]byte(appendix))
	return appendix + "\n" + instruction, hex.EncodeToString(sum[:])
}

func (policy benchmarkPolicy) classifyHostResult(result port.HostProbeResult, fixture fixturecontract.Fixture) fixturecontract.EpisodeReport {
	if !result.Completed {
		return policy.incompleteHostResult(result, fixture, result.Cause, result.Code, result.EvidenceSource)
	}
	classification, err := fixturecontract.ParseClassification(result.Classification)
	if err != nil || result.EvidenceID == "" || !toolconformancedomain.ValidEvidenceID(result.EvidenceID) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if result.CallCount == 0 || classification == fixturecontract.Classification(fixturecontract.NoCall) {
		return policy.incompleteHostResult(result, fixture, "unknown", "no_call", result.Host+"_runner")
	}
	if (result.CallCount > 1) != (classification == fixturecontract.Classification(fixturecontract.MultipleCalls)) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	var arguments any
	if err := json.Unmarshal([]byte(result.CanonicalArgumentsJSON), &arguments); err != nil {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	diagnostics := []fixturecontract.Diagnostic{}
	if err := json.Unmarshal([]byte(result.DiagnosticsJSON), &diagnostics); err != nil {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if diagnostics == nil {
		diagnostics = []fixturecontract.Diagnostic{}
	}
	toolconformancedomain.SortDiagnostics(diagnostics)
	if probeContradictsCanonical(result, classification, diagnostics) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	return policy.completedHostResult(result, classification, arguments, diagnostics)
}

// probeContradictsCanonical reports whether the classification disagrees with
// the canonical validity or the diagnostics the probe returned.
func probeContradictsCanonical(result port.HostProbeResult, classification fixturecontract.Classification, diagnostics []fixturecontract.Diagnostic) bool {
	valid := classification == fixturecontract.Classification(fixturecontract.ExactValid) || classification == fixturecontract.Classification(fixturecontract.ValidButSemanticallyDifferent)
	if valid && (!result.CanonicalValid || len(diagnostics) != 0) {
		return true
	}
	return toolconformancedomain.SchemaDriftClassification(classification) && result.CanonicalValid
}

func (policy benchmarkPolicy) completedHostResult(result port.HostProbeResult, classification fixturecontract.Classification, arguments any, diagnostics []fixturecontract.Diagnostic) fixturecontract.EpisodeReport {
	evidence := []failurecausecontract.Evidence{}
	failed := classification != fixturecontract.Classification(fixturecontract.ExactValid)
	if failed {
		cause := failurecausecontract.Model
		if result.AdvertisedValid && !result.CanonicalValid {
			cause = failurecausecontract.ContractInput
		}
		evidence = append(evidence, failurecausecontract.Evidence{Cause: cause, Code: string(classification), Source: "tool_conformance"})
	}
	causeResult := policy.classify(failed, evidence)
	return fixturecontract.EpisodeReport{
		Status:               fixturecontract.EpisodeCompleted,
		Host:                 result.Host,
		HostVersion:          result.HostVersion,
		RequestedModel:       result.RequestedModel,
		ObservedModel:        result.ObservedModel,
		FixtureID:            result.FixtureID,
		SchemaSHA256:         result.SchemaSHA256,
		Profile:              result.Profile,
		Attempt:              result.Attempt,
		DurationMS:           result.DurationMS,
		SessionStartObserved: result.SessionStartObserved,
		PreToolUseObserved:   result.PreToolUseObserved,
		AmbientToolCount:     result.AmbientToolCount,
		CallCount:            result.CallCount,
		ResponseSHA256:       result.ResponseSHA256,
		ExitCode:             result.ExitCode,
		RawArgumentsSHA256:   result.RawArgumentsSHA256,
		EvidenceID:           result.EvidenceID,
		CanonicalArguments:   arguments,
		Classification:       classification,
		AdvertisedValid:      result.AdvertisedValid,
		CanonicalValid:       result.CanonicalValid,
		Diagnostics:          diagnostics,
		DiagnosticSignature:  toolconformancedomain.DiagnosticSignature(classification, diagnostics),
		FailureCause:         causeResult.Cause,
		FailureCauseReason:   causeResult.Reason,
		FailureCauseEvidence: causeResult.Evidence,
	}
}

func (policy benchmarkPolicy) incompleteHostResult(result port.HostProbeResult, fixture fixturecontract.Fixture, cause, code, source string) fixturecontract.EpisodeReport {
	episode := policy.incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, cause, code, source)
	episode.ObservedModel = result.ObservedModel
	episode.DurationMS = result.DurationMS
	episode.SessionStartObserved = result.SessionStartObserved
	episode.PreToolUseObserved = result.PreToolUseObserved
	episode.AmbientToolCount = result.AmbientToolCount
	episode.CallCount = result.CallCount
	episode.ResponseSHA256 = result.ResponseSHA256
	episode.ExitCode = result.ExitCode
	return episode
}

func (policy benchmarkPolicy) validCompletedEpisode(episode fixturecontract.EpisodeReport, expected toolconformancedomain.CompletedEpisodeExpectation) bool {
	return toolconformancedomain.ValidCompletedEpisode(episode, expected)
}
func (policy benchmarkPolicy) incompleteEpisode(host, version string, fixture fixturecontract.Fixture, profile, model string, attempt int, cause, code, source string) fixturecontract.EpisodeReport {
	parsedCause := failurecausecontract.Cause(cause)
	if parsedCause != failurecausecontract.HarnessEnvironment && parsedCause != failurecausecontract.Transport && parsedCause != failurecausecontract.ContractInput && parsedCause != failurecausecontract.Model {
		parsedCause = failurecausecontract.Unknown
	}
	if source == "" {
		source = "tool_conformance"
	}
	result := policy.classify(true, []failurecausecontract.Evidence{{Cause: parsedCause, Code: code, Source: source}})
	return fixturecontract.EpisodeReport{
		Status:               fixturecontract.EpisodeIncomplete,
		Host:                 host,
		HostVersion:          version,
		RequestedModel:       model,
		ObservedModel:        "",
		FixtureID:            fixture.ID,
		SchemaSHA256:         fixture.SchemaSHA256,
		Profile:              profile,
		Attempt:              attempt,
		Diagnostics:          []fixturecontract.Diagnostic{},
		FailureCause:         result.Cause,
		FailureCauseReason:   result.Reason,
		FailureCauseEvidence: result.Evidence,
	}
}

func countReport(report fixturecontract.BenchmarkReport) fixturecontract.BenchmarkCounts {
	counts := fixturecontract.BenchmarkCounts{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			counts.Attempts++
			if episode.Status != "completed" {
				noCall := false
				for _, evidence := range episode.FailureCauseEvidence {
					if evidence.Code == "no_call" {
						noCall = true
						break
					}
				}
				if noCall {
					counts.NoCalls++
				} else {
					switch episode.FailureCause {
					case failurecausecontract.HarnessEnvironment:
						counts.EnvironmentFailures++
					case failurecausecontract.Transport:
						counts.TransportFailures++
					}
				}
				continue
			}
			counts.Completed++
			counts.ModelDenominator++
			if episode.Classification == fixturecontract.Classification(fixturecontract.ValidButSemanticallyDifferent) {
				counts.ValidSemanticDifferences++
			}
			if toolconformancedomain.SchemaDriftClassification(episode.Classification) {
				counts.SchemaDriftObservations++
			}
		}
	}
	return counts
}

func modelForHost(models map[string]string, host string) string {
	if model := strings.TrimSpace(models[host]); model != "" {
		return model
	}
	return "default"
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
