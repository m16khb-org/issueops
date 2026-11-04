package toolconformance

import (
	"encoding/json"
	"fmt"

	"issueops/internal/contract/failurecause"
)

// FixtureManifestVersion and ReportSchemaVersion are the supported schema versions of the fixture manifest
// and of the benchmark report. The remaining constants are the Classification values a tool call can receive.
const (
	FixtureManifestVersion        = 1
	ReportSchemaVersion           = 2
	ExactValid                    = "exact_valid"
	UnknownKey                    = "unknown_key"
	CoercibleTypeDrift            = "coercible_type_drift"
	NoncoercibleTypeDrift         = "noncoercible_type_drift"
	ValidButSemanticallyDifferent = "valid_but_semantically_different"
	InvalidJSON                   = "invalid_json"
	NoCall                        = "no_call"
	MultipleCalls                 = "multiple_calls"
	MissingRequired               = "missing_required"
	EnumMismatch                  = "enum_mismatch"
)

// ToolDescriptor is a tool as advertised to the model: its name and JSON input schema.
type ToolDescriptor struct {
	Name        string
	InputSchema map[string]any
}

// Diagnostic describes one mismatch between a tool call and its schema: the JSON path, a stable code, and
// the expected and actual values.
type Diagnostic struct {
	Path     string `json:"path"`
	Code     string `json:"code"`
	Expected string `json:"expected"`
	Actual   string `json:"actual"`
}

// CallObservation is what the harness saw of a model's tool call: the raw argument bytes and how many
// calls were made.
type CallObservation struct {
	RawArguments []byte
	CallCount    int
}

// Classification is the verdict category for one tool call, such as ExactValid or UnknownKey.
type Classification string

// CaseResult is the outcome of checking one call: its Classification, whether the arguments are valid for the
// advertised and canonical schemas, and the Diagnostics explaining any mismatch.
type CaseResult struct {
	Classification  Classification `json:"classification"`
	AdvertisedValid bool           `json:"advertised_valid"`
	CanonicalValid  bool           `json:"canonical_valid"`
	Diagnostics     []Diagnostic   `json:"diagnostics"`
}

// BaselineCase is a fixed payload with the Classification it must produce, used to verify the classifier
// itself before judging model output.
type BaselineCase struct {
	FixtureID              string         `json:"fixture_id"`
	PayloadClass           string         `json:"payload_class"`
	ExpectedClassification Classification `json:"expected_classification"`
	Arguments              map[string]any `json:"arguments"`
}

// GateDecision is the benchmark's recommendation on whether schema hardening is warranted.
type GateDecision string

// EpisodeStatus says whether a benchmark episode ran to completion.
type EpisodeStatus string

// EpisodeCompleted and EpisodeIncomplete are the valid EpisodeStatus values.
const (
	EpisodeCompleted  EpisodeStatus = "completed"
	EpisodeIncomplete EpisodeStatus = "incomplete"
)

// GateBaselinePassed and the following constants are the valid GateDecision values, from a passing
// baseline through inconclusive or deferred outcomes to authorizing hardening.
const (
	GateBaselinePassed          GateDecision = "baseline_passed"
	GateInconclusive            GateDecision = "inconclusive"
	GateDeferHardening          GateDecision = "defer_hardening"
	GateNeedsReproduction       GateDecision = "needs_reproduction"
	GateAuthorizeHardening      GateDecision = "authorize_hardening"
	GateUnreproducedObservation GateDecision = "unreproduced_observation"
)

// EpisodeReport records one live episode for a fixture: host and model identity, hook observations,
// call count, the raw-argument hash, Classification, Diagnostics and the attributed failure cause.
type EpisodeReport struct {
	Status               EpisodeStatus           `json:"status"`
	Host                 string                  `json:"host"`
	HostVersion          string                  `json:"host_version"`
	RequestedModel       string                  `json:"requested_model"`
	ObservedModel        string                  `json:"observed_model"`
	FixtureID            string                  `json:"fixture_id"`
	SchemaSHA256         string                  `json:"schema_sha256"`
	Profile              string                  `json:"profile"`
	Attempt              int                     `json:"attempt"`
	DurationMS           int64                   `json:"duration_ms"`
	SessionStartObserved bool                    `json:"session_start_observed"`
	PreToolUseObserved   bool                    `json:"pre_tool_use_observed"`
	AmbientToolCount     int                     `json:"ambient_tool_count"`
	CallCount            int                     `json:"call_count"`
	ResponseSHA256       string                  `json:"response_sha256,omitempty"`
	ExitCode             int                     `json:"exit_code"`
	RawArgumentsSHA256   string                  `json:"raw_arguments_sha256,omitempty"`
	EvidenceID           string                  `json:"evidence_id,omitempty"`
	CanonicalArguments   any                     `json:"canonical_arguments,omitempty"`
	Classification       Classification          `json:"classification,omitempty"`
	AdvertisedValid      bool                    `json:"advertised_valid"`
	CanonicalValid       bool                    `json:"canonical_valid"`
	Diagnostics          []Diagnostic            `json:"diagnostics"`
	DiagnosticSignature  string                  `json:"diagnostic_signature,omitempty"`
	FailureCause         failurecause.Cause      `json:"failure_cause"`
	FailureCauseReason   string                  `json:"failure_cause_reason"`
	FailureCauseEvidence []failurecause.Evidence `json:"failure_cause_evidence"`
}

// HostStatus is the benchmark verdict for one host.
type HostStatus string

// HostStatusSupported and the following constants are the valid HostStatus values.
const (
	HostStatusSupported   HostStatus = "supported"
	HostStatusUnsupported HostStatus = "unsupported"
	HostStatusUnavailable HostStatus = "unavailable"
	HostStatusNotRun      HostStatus = "not-run"
)

// HostEvidence lists what was established for a host: installation, preflight, mock-extension
// verification and whether a live run was attempted and verified.
type HostEvidence struct {
	Installed             bool   `json:"installed"`
	PreflightReady        bool   `json:"preflight_ready"`
	MockExtensionVerified bool   `json:"mock_extension_verified"`
	LiveAttempted         bool   `json:"live_attempted"`
	LiveVerified          bool   `json:"live_verified"`
	StatusReason          string `json:"status_reason,omitempty"`
}

// HostReport is the benchmark result for one host: its HostStatus, HostEvidence, model identity, attempt
// counts and episode reports.
type HostReport struct {
	Status            HostStatus      `json:"status"`
	Evidence          HostEvidence    `json:"evidence"`
	Host              string          `json:"host"`
	Version           string          `json:"version"`
	RequestedModel    string          `json:"requested_model"`
	ObservedModel     string          `json:"observed_model"`
	AttemptCount      int             `json:"attempt_count"`
	CompletedEpisodes int             `json:"completed_episodes"`
	Cases             []EpisodeReport `json:"cases"`
}

// BenchmarkCounts aggregates episode outcomes across hosts, separating environment and transport failures
// from model-attributable results counted in ModelDenominator.
type BenchmarkCounts struct {
	Attempts                 int `json:"attempts"`
	Completed                int `json:"completed"`
	ModelDenominator         int `json:"model_denominator"`
	EnvironmentFailures      int `json:"environment_failures"`
	TransportFailures        int `json:"transport_failures"`
	NoCalls                  int `json:"no_calls"`
	ValidSemanticDifferences int `json:"valid_semantic_differences"`
	SchemaDriftObservations  int `json:"schema_drift_observations"`
}

// GateReport is the GateDecision with the next reproduction target and the confirmed diagnostic signature
// and its count, when any.
type GateReport struct {
	Decision               GateDecision `json:"decision"`
	NextReproductionTarget string       `json:"next_reproduction_target,omitempty"`
	ConfirmedSignature     string       `json:"confirmed_signature,omitempty"`
	ConfirmedCount         int          `json:"confirmed_count,omitempty"`
}

type benchmarkEvidence struct {
	ReportPath              string `json:"report_path,omitempty"`
	CandidateRegressionPath string `json:"candidate_regression_path,omitempty"`
	TrackedFixturePath      string `json:"tracked_fixture_path,omitempty"`
}

// BenchmarkReport is the top-level benchmark result: run identity, profile, aggregate counts, the gate
// decision, per-host reports and warnings.
type BenchmarkReport struct {
	OK            bool              `json:"ok"`
	SchemaVersion int               `json:"schema_version"`
	RunID         string            `json:"run_id"`
	Profile       string            `json:"profile"`
	ProfileSHA256 string            `json:"profile_sha256,omitempty"`
	CaseCount     int               `json:"case_count"`
	Counts        BenchmarkCounts   `json:"counts"`
	Gate          GateReport        `json:"gate"`
	Hosts         []HostReport      `json:"hosts"`
	Warnings      []string          `json:"warnings"`
	Evidence      benchmarkEvidence `json:"evidence"`
}

// ParseEpisodeStatus converts value to an EpisodeStatus, returning an error for an unknown status.
func ParseEpisodeStatus(value string) (EpisodeStatus, error) {
	switch EpisodeStatus(value) {
	case EpisodeCompleted, EpisodeIncomplete:
		return EpisodeStatus(value), nil
	default:
		return "", fmt.Errorf("unknown episode status %q", value)
	}
}

// UnmarshalJSON decodes a JSON string into an EpisodeStatus and rejects unknown statuses.
func (value *EpisodeStatus) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseEpisodeStatus(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// ValidGateDecision reports whether value is a known GateDecision.
func ValidGateDecision(value GateDecision) bool {
	switch value {
	case GateBaselinePassed, GateInconclusive, GateDeferHardening, GateNeedsReproduction, GateAuthorizeHardening, GateUnreproducedObservation:
		return true
	default:
		return false
	}
}

// ParseClassification converts value to a Classification, returning an error for an unknown classification.
func ParseClassification(value string) (Classification, error) {
	switch value {
	case ExactValid, ValidButSemanticallyDifferent, UnknownKey, CoercibleTypeDrift, NoncoercibleTypeDrift, InvalidJSON, NoCall, MultipleCalls, MissingRequired, EnumMismatch:
		return Classification(value), nil
	default:
		return "", fmt.Errorf("unknown classification %q", value)
	}
}

// UnmarshalJSON decodes a JSON string into a Classification and rejects unknown values.
func (value *Classification) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseClassification(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// UnmarshalJSON decodes a JSON string into a GateDecision and rejects unknown decisions.
func (value *GateDecision) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	parsed, err := ParseGateDecision(raw)
	if err != nil {
		return err
	}
	*value = parsed
	return nil
}

// ParseGateDecision converts value to a GateDecision, returning an error for an unknown decision.
func ParseGateDecision(value string) (GateDecision, error) {
	decision := GateDecision(value)
	if !ValidGateDecision(decision) {
		return "", fmt.Errorf("unknown gate decision %q", value)
	}
	return decision, nil
}

// Fixture ties a probe tool to the source tool and schema hash it derives from, with the arguments a
// conforming call is expected to produce.
type Fixture struct {
	ID                string         `json:"id"`
	ProbeTool         string         `json:"probe_tool"`
	SourceTool        string         `json:"source_tool"`
	SchemaSHA256      string         `json:"schema_sha256"`
	ExpectedArguments map[string]any `json:"expected_arguments"`
}

// RegressionFixture is a confirmed, replayable failure: the canonical arguments and the classification,
// diagnostics, handler call count and unchanged-state expectation that replaying them must reproduce.
type RegressionFixture struct {
	SchemaVersion               int            `json:"schema_version"`
	FixtureID                   string         `json:"fixture_id"`
	SourceTool                  string         `json:"source_tool"`
	ProbeTool                   string         `json:"probe_tool"`
	SourceSchemaSHA256          string         `json:"source_schema_sha256"`
	Host                        string         `json:"host"`
	HostVersion                 string         `json:"host_version"`
	ModelLabel                  string         `json:"model_label"`
	CanonicalArguments          any            `json:"canonical_arguments"`
	RawArgumentsSHA256          string         `json:"raw_arguments_sha256"`
	ExpectedClassification      Classification `json:"expected_classification"`
	ExpectedDiagnostics         []Diagnostic   `json:"expected_diagnostics"`
	ExpectedDiagnosticSignature string         `json:"expected_diagnostic_signature"`
	ConfirmedEvidenceIDs        []string       `json:"confirmed_evidence_ids"`
	ExpectedHandlerCallCount    int            `json:"expected_handler_call_count"`
	ExpectedFinalResult         map[string]any `json:"expected_final_result"`
	ExpectedStateUnchanged      bool           `json:"expected_state_unchanged"`
}

// ReplayResult is the outcome of replaying a RegressionFixture: classification, diagnostics, handler call
// count, final result and the state hashes before and after the replay.
type ReplayResult struct {
	OK                  bool           `json:"ok"`
	Classification      Classification `json:"classification"`
	Diagnostics         []Diagnostic   `json:"diagnostics"`
	DiagnosticSignature string         `json:"diagnostic_signature"`
	HandlerCalls        int            `json:"handler_calls"`
	FinalResult         map[string]any `json:"final_result"`
	StateBeforeSHA256   string         `json:"state_before_sha256"`
	StateAfterSHA256    string         `json:"state_after_sha256"`
}
