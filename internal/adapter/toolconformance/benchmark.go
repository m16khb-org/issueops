package toolconformance

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	failurecausecontract "issueops/internal/contract/failurecause"
	issueopscontract "issueops/internal/contract/issueops"
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
	Runners map[string]port.HostProbeRunner
	Now     func() time.Time
	Token   func() string
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
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Token == nil {
		deps.Token = randomToken
	}
	fixtures, _, err := LoadManifest(descriptors)
	if err != nil {
		return BenchmarkReport{}, err
	}
	selected, err := selectFixturePairs(request.Hosts, fixtures, request.Only)
	if err != nil {
		return BenchmarkReport{}, err
	}
	if err := validatePreviousSelection(request.Previous, selected); err != nil {
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
			previousHost, err := resumableHostReport(request.Previous, host, hostReport.RequestedModel, preflight.Version, request.Profile, fixtures, selected, request.TargetCompleted)
			if err != nil {
				return BenchmarkReport{}, err
			}
			if previousHost != nil {
				for _, episode := range previousHost.Cases {
					fixture, selectedPair := selectedFixtureForPair(selected, host, episode.FixtureID)
					if !selectedPair || episode.Status != EpisodeCompleted {
						continue
					}
					expectation := completedEpisodeExpectation{
						Host: host, HostVersion: preflight.Version, RequestedModel: hostReport.RequestedModel,
						Profile: request.Profile, Fixture: fixture, Attempt: episode.Attempt,
					}
					if !validCompletedEpisode(episode, expectation) {
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
			completed := completedForFixture(hostReport.Cases, fixture.ID)
			if !preflight.Ready {
				episode := incompleteEpisode(host, preflight.Version, fixture, request.Profile, hostReport.RequestedModel, 0, preflight.Cause, preflight.Code, preflight.EvidenceSource)
				hostReport.Cases = append(hostReport.Cases, episode)
				continue
			}
			newAttemptLimit := (request.TargetCompleted - completed) * request.MaxAttemptsPerCase
			for newAttempts := 0; completed < request.TargetCompleted && newAttempts < newAttemptLimit; newAttempts++ {
				hostReport.Evidence.LiveAttempted = true
				attempt := attemptsForFixture(hostReport.Cases, fixture.ID) + 1
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
				episode := classifyHostResult(runResult, fixture)
				expectation := completedEpisodeExpectation{
					Host: host, HostVersion: preflight.Version, RequestedModel: hostReport.RequestedModel,
					Profile: request.Profile, Fixture: fixture, Attempt: attempt,
				}
				if episode.Status == EpisodeCompleted && !validCompletedEpisode(episode, expectation) {
					episode = incompleteEpisode(host, preflight.Version, fixture, request.Profile, hostReport.RequestedModel, attempt, "transport", "probe_result_invalid", host+"_runner")
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
		hostReport.CompletedEpisodes = countCompleted(hostReport.Cases)
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
	report.Gate = decideGate(report, selected, request.TargetCompleted)
	if report.Gate.Decision == GateInconclusive {
		report.OK = false
	}
	if report.Counts.Attempts > 0 && float64(report.Counts.EnvironmentFailures+report.Counts.TransportFailures)/float64(report.Counts.Attempts) > 0.05 {
		report.Warnings = append(report.Warnings, "environment_transport_failure_rate_above_5_percent")
	}
	return report, nil
}

type fixturePair struct {
	Host    string
	Fixture Fixture
}

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

func classifyHostResult(result port.HostProbeResult, fixture Fixture) EpisodeReport {
	if !result.Completed {
		return incompleteHostResult(result, fixture, result.Cause, result.Code, result.EvidenceSource)
	}
	classification, err := ParseClassification(result.Classification)
	if err != nil || result.EvidenceID == "" || !validEvidenceID(result.EvidenceID) {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if result.CallCount == 0 || classification == Classification(NoCall) {
		return incompleteHostResult(result, fixture, "unknown", "no_call", result.Host+"_runner")
	}
	if (result.CallCount > 1) != (classification == Classification(MultipleCalls)) {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	var arguments any
	if err := json.Unmarshal([]byte(result.CanonicalArgumentsJSON), &arguments); err != nil {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	diagnostics := []Diagnostic{}
	if err := json.Unmarshal([]byte(result.DiagnosticsJSON), &diagnostics); err != nil {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if diagnostics == nil {
		diagnostics = []Diagnostic{}
	}
	sortDiagnostics(diagnostics)
	if (classification == Classification(ExactValid) || classification == Classification(ValidButSemanticallyDifferent)) && (!result.CanonicalValid || len(diagnostics) != 0) {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
	}
	if schemaDriftClassification(classification) && result.CanonicalValid {
		return incompleteHostResult(result, fixture, "transport", "probe_result_invalid", result.Host+"_runner")
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
	causeResult := ClassifyFailureCause(failed, evidence)
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

func incompleteHostResult(result port.HostProbeResult, fixture Fixture, cause, code, source string) EpisodeReport {
	episode := incompleteEpisode(result.Host, result.HostVersion, fixture, result.Profile, result.RequestedModel, result.Attempt, cause, code, source)
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

func validEvidenceID(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

type completedEpisodeExpectation struct {
	Host           string
	HostVersion    string
	RequestedModel string
	Profile        string
	Fixture        Fixture
	Attempt        int
}

func validCompletedEpisode(episode EpisodeReport, expected completedEpisodeExpectation) bool {
	if episode.Status != EpisodeCompleted || episode.Host != expected.Host || episode.HostVersion != expected.HostVersion ||
		episode.RequestedModel != expected.RequestedModel || strings.TrimSpace(episode.ObservedModel) == "" || strings.TrimSpace(episode.ObservedModel) != episode.ObservedModel ||
		len(episode.ObservedModel) > 256 || strings.ContainsAny(episode.ObservedModel, "\r\n") ||
		episode.FixtureID != expected.Fixture.ID || episode.SchemaSHA256 != expected.Fixture.SchemaSHA256 ||
		episode.Profile != expected.Profile || episode.Attempt != expected.Attempt || episode.Attempt < 1 {
		return false
	}
	if episode.DurationMS <= 0 || !episode.SessionStartObserved || episode.PreToolUseObserved || episode.ExitCode != 0 || episode.AmbientToolCount != 1 || episode.CallCount != 1 {
		return false
	}
	if !validEvidenceID(episode.EvidenceID) || !validEvidenceID(episode.RawArgumentsSHA256) || !validEvidenceID(episode.ResponseSHA256) || episode.CanonicalArguments == nil {
		return false
	}
	if _, err := ParseClassification(string(episode.Classification)); err != nil || episode.Classification == Classification(NoCall) || episode.Classification == Classification(MultipleCalls) || episode.Classification == Classification(InvalidJSON) {
		return false
	}
	orderedDiagnostics := append([]Diagnostic{}, episode.Diagnostics...)
	sortDiagnostics(orderedDiagnostics)
	if !reflect.DeepEqual(orderedDiagnostics, episode.Diagnostics) || !validEvidenceID(episode.DiagnosticSignature) || episode.DiagnosticSignature != DiagnosticSignature(episode.Classification, episode.Diagnostics) {
		return false
	}
	for _, diagnostic := range episode.Diagnostics {
		if diagnostic.Code == "" || len(diagnostic.Path) > 512 || len(diagnostic.Code) > 128 || len(diagnostic.Expected) > 1024 || len(diagnostic.Actual) > 1024 {
			return false
		}
	}
	validClassification := episode.Classification == Classification(ExactValid) || episode.Classification == Classification(ValidButSemanticallyDifferent)
	if validClassification && (!episode.AdvertisedValid || !episode.CanonicalValid || len(episode.Diagnostics) != 0) {
		return false
	}
	if schemaDriftClassification(episode.Classification) && episode.CanonicalValid {
		return false
	}
	evidence := []failurecausecontract.Evidence{}
	failed := episode.Classification != Classification(ExactValid)
	if failed {
		cause := failurecausecontract.Model
		if episode.AdvertisedValid && !episode.CanonicalValid {
			cause = failurecausecontract.ContractInput
		}
		evidence = append(evidence, failurecausecontract.Evidence{Cause: cause, Code: string(episode.Classification), Source: "tool_conformance"})
	}
	cause := ClassifyFailureCause(failed, evidence)
	return episode.FailureCause == cause.Cause && episode.FailureCauseReason == cause.Reason && reflect.DeepEqual(episode.FailureCauseEvidence, cause.Evidence)
}
func incompleteEpisode(host, version string, fixture Fixture, profile, model string, attempt int, cause, code, source string) EpisodeReport {
	parsedCause := failurecausecontract.Cause(cause)
	if parsedCause != failurecausecontract.HarnessEnvironment && parsedCause != failurecausecontract.Transport && parsedCause != failurecausecontract.ContractInput && parsedCause != failurecausecontract.Model {
		parsedCause = failurecausecontract.Unknown
	}
	if source == "" {
		source = "tool_conformance"
	}
	result := ClassifyFailureCause(true, []failurecausecontract.Evidence{{Cause: parsedCause, Code: code, Source: source}})
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
	value := struct {
		Classification Classification `json:"classification"`
		Diagnostics    []Diagnostic   `json:"diagnostics"`
	}{classification, append([]Diagnostic(nil), diagnostics...)}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func decideGate(report BenchmarkReport, selected []fixturePair, target int) GateReport {
	for _, pair := range selected {
		if completedForPair(report, pair.Host, pair.Fixture.ID) < target {
			return GateReport{Decision: GateInconclusive}
		}
	}
	type signatureCount struct {
		Host      string
		FixtureID string
		Signature string
		Count     int
	}
	counts := map[string]*signatureCount{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			if episode.Status != "completed" || !schemaDriftClassification(episode.Classification) {
				continue
			}
			key := host.Host + "\x00" + episode.FixtureID + "\x00" + episode.DiagnosticSignature
			if counts[key] == nil {
				counts[key] = &signatureCount{Host: host.Host, FixtureID: episode.FixtureID, Signature: episode.DiagnosticSignature}
			}
			counts[key].Count++
		}
	}
	ordered := make([]*signatureCount, 0, len(counts))
	for _, count := range counts {
		ordered = append(ordered, count)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Host != ordered[j].Host {
			return ordered[i].Host < ordered[j].Host
		}
		if ordered[i].FixtureID != ordered[j].FixtureID {
			return ordered[i].FixtureID < ordered[j].FixtureID
		}
		return ordered[i].Signature < ordered[j].Signature
	})
	for _, count := range ordered {
		if count.Count >= 2 {
			return GateReport{Decision: GateAuthorizeHardening, ConfirmedSignature: count.Signature, ConfirmedCount: count.Count}
		}
	}
	if len(ordered) == 0 {
		return GateReport{Decision: GateDeferHardening}
	}
	if target >= 20 {
		return GateReport{Decision: GateUnreproducedObservation}
	}
	return GateReport{Decision: GateNeedsReproduction, NextReproductionTarget: ordered[0].Host + ":" + ordered[0].FixtureID}
}

func schemaDriftClassification(classification Classification) bool {
	switch string(classification) {
	case UnknownKey, CoercibleTypeDrift, NoncoercibleTypeDrift, InvalidJSON, MissingRequired, EnumMismatch:
		return true
	default:
		return false
	}
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
			if schemaDriftClassification(episode.Classification) {
				counts.SchemaDriftObservations++
			}
		}
	}
	return counts
}

func resumableHostReport(previous *BenchmarkReport, host, requestedModel, hostVersion, profile string, fixtures []Fixture, selected []fixturePair, targetCompleted int) (*HostReport, error) {
	if previous == nil {
		return nil, nil
	}
	var matched *HostReport
	for index := range previous.Hosts {
		candidate := &previous.Hosts[index]
		if candidate.Host != host {
			continue
		}
		matched = candidate
		break
	}
	if matched == nil {
		return nil, nil
	}
	if matched.Version != hostVersion || matched.RequestedModel != requestedModel {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	if matched.AttemptCount != len(matched.Cases) || matched.CompletedEpisodes != countCompleted(matched.Cases) || matched.AttemptCount < 0 || matched.CompletedEpisodes < 0 {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	fixturesByID := make(map[string]Fixture, len(fixtures))
	for _, fixture := range fixtures {
		fixturesByID[fixture.ID] = fixture
	}
	seenIdentities := map[string]bool{}
	seenEvidence := map[string]bool{}
	observedModel := ""
	for _, episode := range matched.Cases {
		identity := episode.Host + "\x00" + episode.FixtureID + "\x00" + fmt.Sprint(episode.Attempt)
		if seenIdentities[identity] {
			return nil, fmt.Errorf("duplicate_previous_episode_identity")
		}
		seenIdentities[identity] = true
		fixture, knownFixture := fixturesByID[episode.FixtureID]
		if episode.Host != host || episode.HostVersion != hostVersion || episode.RequestedModel != requestedModel || episode.Profile != profile ||
			!knownFixture || episode.SchemaSHA256 != fixture.SchemaSHA256 || episode.Attempt < 1 ||
			(episode.Status != EpisodeCompleted && episode.Status != EpisodeIncomplete) {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
		if episode.ObservedModel != "" {
			if strings.TrimSpace(episode.ObservedModel) != episode.ObservedModel || len(episode.ObservedModel) > 256 || strings.ContainsAny(episode.ObservedModel, "\r\n") ||
				(observedModel != "" && episode.ObservedModel != observedModel) {
				return nil, fmt.Errorf("invalid_previous_episode_evidence")
			}
			observedModel = episode.ObservedModel
		}
		if episode.Status != EpisodeCompleted {
			continue
		}
		if !validCompletedEpisode(episode, completedEpisodeExpectation{
			Host: host, HostVersion: hostVersion, RequestedModel: requestedModel,
			Profile: profile, Fixture: fixture, Attempt: episode.Attempt,
		}) || episode.ObservedModel != matched.ObservedModel {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
		if seenEvidence[episode.EvidenceID] {
			return nil, fmt.Errorf("duplicate_previous_episode_evidence")
		}
		seenEvidence[episode.EvidenceID] = true
	}
	hasCompleted := matched.CompletedEpisodes > 0
	hasLiveAttempts := len(matched.Cases) > 0
	if !matched.Evidence.Installed || !matched.Evidence.PreflightReady || matched.Evidence.LiveAttempted != hasLiveAttempts ||
		matched.Evidence.LiveVerified != hasCompleted || !hasLiveAttempts || matched.ObservedModel != observedModel {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	if hasCompleted {
		if matched.Status != issueopscontract.StatusSupported || strings.TrimSpace(matched.ObservedModel) == "" || matched.Evidence.StatusReason != "" {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
	} else if matched.Status != issueopscontract.StatusUnavailable || matched.Evidence.StatusReason != "live_probe_incomplete" {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	for _, pair := range selected {
		if pair.Host == host && completedForFixture(matched.Cases, pair.Fixture.ID) > targetCompleted {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
	}
	return matched, nil
}

func validatePreviousSelection(previous *BenchmarkReport, selected []fixturePair) error {
	if previous == nil {
		return nil
	}
	seenHosts := make(map[string]bool, len(previous.Hosts))
	for _, hostReport := range previous.Hosts {
		if hostReport.Host == "" || seenHosts[hostReport.Host] {
			return fmt.Errorf("invalid_previous_report_identity")
		}
		seenHosts[hostReport.Host] = true
	}
	for _, hostReport := range previous.Hosts {
		selectedHost := false
		for _, pair := range selected {
			if pair.Host == hostReport.Host {
				selectedHost = true
				break
			}
		}
		if !selectedHost {
			return fmt.Errorf("invalid_previous_episode_selection")
		}
		for _, episode := range hostReport.Cases {
			if _, selectedPair := selectedFixtureForPair(selected, hostReport.Host, episode.FixtureID); !selectedPair {
				return fmt.Errorf("invalid_previous_episode_selection")
			}
		}
	}
	return nil
}

func selectedFixtureForPair(pairs []fixturePair, host, fixture string) (Fixture, bool) {
	for _, pair := range pairs {
		if pair.Host == host && pair.Fixture.ID == fixture {
			return pair.Fixture, true
		}
	}
	return Fixture{}, false
}

func completedForPair(report BenchmarkReport, host, fixture string) int {
	for _, hostReport := range report.Hosts {
		if hostReport.Host == host {
			return completedForFixture(hostReport.Cases, fixture)
		}
	}
	return 0
}

func completedForFixture(episodes []EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture && episode.Status == "completed" {
			count++
		}
	}
	return count
}

func attemptsForFixture(episodes []EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture {
			count++
		}
	}
	return count
}

func countCompleted(episodes []EpisodeReport) int {
	count := 0
	for _, episode := range episodes {
		if episode.Status == "completed" {
			count++
		}
	}
	return count
}

func modelForHost(models map[string]string, host string) string {
	if model := strings.TrimSpace(models[host]); model != "" {
		return model
	}
	return "default"
}

func randomToken() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		sum := sha256.Sum256([]byte(time.Now().UTC().String()))
		return hex.EncodeToString(sum[:16])
	}
	return hex.EncodeToString(value)
}

func mustJSON(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
