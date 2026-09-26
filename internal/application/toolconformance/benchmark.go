package toolconformance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	failurecausecontract "issueops/internal/contract/failurecause"
	issueopscontract "issueops/internal/contract/issueops"
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
	Previous           *BenchmarkReport
}

type LiveBenchmarkDependencies struct {
	Runners      map[string]port.HostProbeRunner
	Now          func() time.Time
	Token        func() string
	LoadManifest func([]ToolDescriptor) ([]Fixture, []BaselineCase, error)
	Classify     func(bool, []failurecausecontract.Evidence) failurecausecontract.Result
}

func RunLiveBenchmark(ctx context.Context, request LiveBenchmarkRequest, descriptors []ToolDescriptor, deps LiveBenchmarkDependencies) (BenchmarkReport, error) {
	if err := validateLiveRequest(request); err != nil {
		return BenchmarkReport{}, err
	}
	if request.Previous != nil {
		if request.Previous.SchemaVersion != ReportSchemaVersion {
			return BenchmarkReport{}, fmt.Errorf("unsupported_previous_report_schema:%d", request.Previous.SchemaVersion)
		}
		if request.Previous.Profile != request.Profile {
			return BenchmarkReport{}, fmt.Errorf("invalid_previous_report_identity")
		}
	}
	policy := benchmarkPolicy{classify: deps.Classify}
	fixtures, _, err := deps.LoadManifest(descriptors)
	if err != nil {
		return BenchmarkReport{}, err
	}
	selected, err := selectFixturePairs(request.Hosts, fixtures, request.Only)
	if err != nil {
		return BenchmarkReport{}, err
	}
	if err := toolconformancedomain.ValidatePreviousSelection(request.Previous, selected); err != nil {
		return BenchmarkReport{}, err
	}
	report := BenchmarkReport{
		OK:            true,
		SchemaVersion: ReportSchemaVersion,
		RunID:         request.RunID,
		Profile:       request.Profile,
		CaseCount:     len(selected),
		Hosts:         []HostReport{},
		Warnings:      []string{},
	}
	_, report.ProfileSHA256 = BuildEpisodePrompt(selected[0].Fixture, request.Profile)
	if report.RunID == "" {
		report.RunID = deps.Now().UTC().Format("20060102T150405.000000000Z")
	}
	models := request.Models
	seenEvidenceIDs := map[string]bool{}
	for _, host := range request.Hosts {
		runner := deps.Runners[host]
		if runner == nil {
			return BenchmarkReport{}, fmt.Errorf("unsupported_host:%s", host)
		}
		hostReport := HostReport{
			Status: issueopscontract.StatusNotRun,
			Host:   host, RequestedModel: modelForHost(models, host), Cases: []EpisodeReport{},
		}
		preflightRequest := port.HostProbeRequest{HarnessBinary: request.HarnessBinary, Model: hostReport.RequestedModel}
		preflight := runner.Preflight(ctx, preflightRequest)
		hostReport.Version = preflight.Version
		hostReport.Evidence.Installed = preflight.Installed
		hostReport.Evidence.PreflightReady = preflight.Ready
		hostReport.Evidence.MockExtensionVerified = preflight.MockExtensionVerified
		if preflight.ObservedModel != "" {
			hostReport.ObservedModel = preflight.ObservedModel
		}
		if preflight.Ready {
			previousHost, err := toolconformancedomain.ResumeHostReport(request.Previous, host, hostReport.RequestedModel, preflight.Version, request.Profile, fixtures, selected, request.TargetCompleted)
			if err != nil {
				return BenchmarkReport{}, err
			}
			if previousHost != nil {
				for _, episode := range previousHost.Cases {
					fixture, selectedPair := toolconformancedomain.SelectedFixtureForPair(selected, host, episode.FixtureID)
					if !selectedPair || episode.Status != EpisodeCompleted {
						continue
					}
					expectation := completedEpisodeExpectation{
						Host: host, HostVersion: preflight.Version, RequestedModel: hostReport.RequestedModel,
						Profile: request.Profile, Fixture: fixture, Attempt: episode.Attempt,
					}
					if !policy.validCompletedEpisode(episode, expectation) {
						return BenchmarkReport{}, fmt.Errorf("invalid_previous_episode_evidence")
					}
					if seenEvidenceIDs[episode.EvidenceID] {
						return BenchmarkReport{}, fmt.Errorf("duplicate_previous_episode_evidence")
					}
					seenEvidenceIDs[episode.EvidenceID] = true
					hostReport.Cases = append(hostReport.Cases, episode)
					hostReport.ObservedModel = episode.ObservedModel
				}
				hostReport.Evidence.LiveAttempted = len(hostReport.Cases) > 0
			}
		}
		if !preflight.Ready {
			hostReport.Evidence.StatusReason = preflight.Code
			if !preflight.Installed || preflight.Code == "version_probe_failed" {
				hostReport.Status = issueopscontract.StatusUnavailable
			} else if preflight.Code == "mock_extension_invalid" {
				hostReport.Status = issueopscontract.StatusUnsupported
			}
		}
		for _, pair := range selected {
			if pair.Host != host {
				continue
			}
			fixture := pair.Fixture
			completed := toolconformancedomain.CompletedForFixture(hostReport.Cases, fixture.ID)
			if !preflight.Ready {
				episode := policy.incompleteEpisode(host, preflight.Version, fixture, request.Profile, hostReport.RequestedModel, 0, preflight.Cause, preflight.Code, preflight.EvidenceSource)
				hostReport.Cases = append(hostReport.Cases, episode)
				continue
			}
			newAttemptLimit := (request.TargetCompleted - completed) * request.MaxAttemptsPerCase
			for newAttempts := 0; completed < request.TargetCompleted && newAttempts < newAttemptLimit; newAttempts++ {
				hostReport.Evidence.LiveAttempted = true
				attempt := toolconformancedomain.AttemptsForFixture(hostReport.Cases, fixture.ID) + 1
				prompt, _ := BuildEpisodePrompt(fixture, request.Profile)
				runResult := runner.Run(ctx, port.HostProbeRequest{
					HarnessBinary:         request.HarnessBinary,
					HostVersion:           preflight.Version,
					FixtureID:             fixture.ID,
					ProbeTool:             fixture.ProbeTool,
					SourceTool:            fixture.SourceTool,
					SchemaSHA256:          fixture.SchemaSHA256,
					ExpectedArgumentsJSON: mustJSON(fixture.ExpectedArguments),
					Prompt:                prompt,
					Model:                 hostReport.RequestedModel,
					Profile:               request.Profile,
					Attempt:               attempt,
					RunToken:              deps.Token(),
				})
				episode := policy.classifyHostResult(runResult, fixture)
				expectation := completedEpisodeExpectation{
					Host: host, HostVersion: preflight.Version, RequestedModel: hostReport.RequestedModel,
					Profile: request.Profile, Fixture: fixture, Attempt: attempt,
				}
				if episode.Status == EpisodeCompleted && !policy.validCompletedEpisode(episode, expectation) {
					episode = policy.incompleteEpisode(host, preflight.Version, fixture, request.Profile, hostReport.RequestedModel, attempt, "transport", "probe_result_invalid", host+"_runner")
				}
				if episode.Status == EpisodeCompleted {
					if seenEvidenceIDs[episode.EvidenceID] {
						return BenchmarkReport{}, fmt.Errorf("duplicate_episode_evidence")
					}
					seenEvidenceIDs[episode.EvidenceID] = true
				}
				if episode.ObservedModel != "" {
					hostReport.ObservedModel = episode.ObservedModel
				}
				hostReport.Cases = append(hostReport.Cases, episode)
				if episode.Status == "completed" {
					completed++
				}
			}
		}
		sort.SliceStable(hostReport.Cases, func(i, j int) bool {
			if hostReport.Cases[i].FixtureID != hostReport.Cases[j].FixtureID {
				return hostReport.Cases[i].FixtureID < hostReport.Cases[j].FixtureID
			}
			return hostReport.Cases[i].Attempt < hostReport.Cases[j].Attempt
		})
		hostReport.AttemptCount = len(hostReport.Cases)
		hostReport.CompletedEpisodes = toolconformancedomain.CountCompleted(hostReport.Cases)
		if hostReport.Evidence.PreflightReady && hostReport.CompletedEpisodes > 0 {
			hostReport.Status = issueopscontract.StatusSupported
			hostReport.Evidence.LiveVerified = true
			hostReport.Evidence.StatusReason = ""
		} else if hostReport.Evidence.LiveAttempted {
			hostReport.Status = issueopscontract.StatusUnavailable
			hostReport.Evidence.StatusReason = "live_probe_incomplete"
		}
		report.Hosts = append(report.Hosts, hostReport)
	}
	report.Counts = countReport(report)
	gatePairs := make([]toolconformancedomain.SelectedPair, 0, len(selected))
	for _, pair := range selected {
		gatePairs = append(gatePairs, toolconformancedomain.SelectedPair{Host: pair.Host, FixtureID: pair.Fixture.ID})
	}
	report.Gate = toolconformancedomain.DecideGate(report, gatePairs, request.TargetCompleted)
	if report.Gate.Decision == GateInconclusive {
		report.OK = false
	}
	if report.Counts.Attempts > 0 && float64(report.Counts.EnvironmentFailures+report.Counts.TransportFailures)/float64(report.Counts.Attempts) > 0.05 {
		report.Warnings = append(report.Warnings, "environment_transport_failure_rate_above_5_percent")
	}
	return report, nil
}

type benchmarkPolicy struct {
	classify func(bool, []failurecausecontract.Evidence) failurecausecontract.Result
}

type fixturePair = toolconformancedomain.SelectedFixturePair

func selectFixturePairs(hosts []string, fixtures []Fixture, only string) ([]fixturePair, error) {
	pairs := []fixturePair{}
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
			pairs = append(pairs, fixturePair{Host: host, Fixture: fixture})
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

func BuildEpisodePrompt(fixture Fixture, profile string) (string, string) {
	instruction := fmt.Sprintf("Call the only allowed MCP tool %s exactly once with these exact JSON arguments. Do not add, remove, rename, or coerce any field. After the tool result, output only captured.\nArguments:\n%s", fixture.ProbeTool, mustJSON(fixture.ExpectedArguments))
	if profile != "context-pressure" {
		return instruction, ""
	}
	unit := "CONFORMANCE_CONTEXT_PRESSURE_FIXED_BLOCK\n"
	appendix := strings.Repeat(unit, contextPressureBytes/len(unit)+1)[:contextPressureBytes]
	sum := sha256.Sum256([]byte(appendix))
	return appendix + "\n" + instruction, hex.EncodeToString(sum[:])
}

func (policy benchmarkPolicy) classifyHostResult(result port.HostProbeResult, fixture Fixture) EpisodeReport {
	if !result.Completed {
		return policy.incompleteHostResult(result, fixture, result.Cause, result.Code, result.EvidenceSource)
	}
	classification, err := ParseClassification(result.Classification)
	if err != nil || result.EvidenceID == "" || !ValidEvidenceID(result.EvidenceID) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if result.CallCount == 0 || classification == Classification(NoCall) {
		return policy.incompleteHostResult(result, fixture, "unknown", "no_call", result.Host+"_runner")
	}
	if (result.CallCount > 1) != (classification == Classification(MultipleCalls)) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	var arguments any
	if err := json.Unmarshal([]byte(result.CanonicalArgumentsJSON), &arguments); err != nil {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	diagnostics := []Diagnostic{}
	if err := json.Unmarshal([]byte(result.DiagnosticsJSON), &diagnostics); err != nil {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	toolconformancedomain.SortDiagnostics(diagnostics)
	if (classification == Classification(ExactValid) || classification == Classification(ValidButSemanticallyDifferent)) && (!result.CanonicalValid || len(diagnostics) != 0) {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if toolconformancedomain.SchemaDriftClassification(classification) && result.CanonicalValid {
		return policy.incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	evidence := []failurecausecontract.Evidence{}
	failed := classification != Classification(ExactValid)
	if failed {
		cause := failurecausecontract.Model
		if result.AdvertisedValid && !result.CanonicalValid {
			cause = failurecausecontract.ContractInput
		}
		evidence = append(evidence, failurecausecontract.Evidence{Cause: cause, Code: string(classification), Source: "tool_conformance"})
	}
	causeResult := policy.classify(failed, evidence)
	return EpisodeReport{
		Status:               EpisodeCompleted,
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
		DiagnosticSignature:  DiagnosticSignature(classification, diagnostics),
		FailureCause:         causeResult.Cause,
		FailureCauseReason:   causeResult.Reason,
		FailureCauseEvidence: causeResult.Evidence,
	}
}

func (policy benchmarkPolicy) incompleteHostResult(result port.HostProbeResult, fixture Fixture, cause, code, source string) EpisodeReport {
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

func ValidEvidenceID(value string) bool { return toolconformancedomain.ValidEvidenceID(value) }

type completedEpisodeExpectation = toolconformancedomain.CompletedEpisodeExpectation

func (policy benchmarkPolicy) validCompletedEpisode(episode EpisodeReport, expected completedEpisodeExpectation) bool {
	return toolconformancedomain.ValidCompletedEpisode(episode, expected)
}
func (policy benchmarkPolicy) incompleteEpisode(host, version string, fixture Fixture, profile, model string, attempt int, cause, code, source string) EpisodeReport {
	parsedCause := failurecausecontract.Cause(cause)
	if parsedCause != failurecausecontract.HarnessEnvironment && parsedCause != failurecausecontract.Transport && parsedCause != failurecausecontract.ContractInput && parsedCause != failurecausecontract.Model {
		parsedCause = failurecausecontract.Unknown
	}
	if source == "" {
		source = "tool_conformance"
	}
	result := policy.classify(true, []failurecausecontract.Evidence{{Cause: parsedCause, Code: code, Source: source}})
	return EpisodeReport{
		Status:               EpisodeIncomplete,
		Host:                 host,
		HostVersion:          version,
		RequestedModel:       model,
		ObservedModel:        "",
		FixtureID:            fixture.ID,
		SchemaSHA256:         fixture.SchemaSHA256,
		Profile:              profile,
		Attempt:              attempt,
		Diagnostics:          []Diagnostic{},
		FailureCause:         result.Cause,
		FailureCauseReason:   result.Reason,
		FailureCauseEvidence: result.Evidence,
	}
}

func DiagnosticSignature(classification Classification, diagnostics []Diagnostic) string {
	return toolconformancedomain.DiagnosticSignature(classification, diagnostics)
}

func countReport(report BenchmarkReport) BenchmarkCounts {
	counts := BenchmarkCounts{}
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
			if episode.Classification == Classification(ValidButSemanticallyDifferent) {
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
