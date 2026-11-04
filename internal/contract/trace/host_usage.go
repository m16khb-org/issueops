package trace

// Input formats accepted by trace analysis; InputFormatIssueOps is the harness's own trace and the rest are host transcript formats.
const (
	InputFormatIssueOps     = "issueops"
	InputFormatClaudeJSON   = "claude-stream"
	InputFormatCodexExec    = "codex-exec"
	InputFormatOmoJSON      = "omo-json"
	InputFormatOmpJSON      = "omp-json"
	InputFormatOpencodeJSON = "opencode-json"
)

// Size limits for host transcript input: HostInputMaxBytes caps the whole input and HostLineMaxBytes a single line.
const (
	HostInputMaxBytes = 16 << 20
	HostLineMaxBytes  = 1 << 20
)

// Usage report vocabularies: how completely the host reported usage (coverage), whether a sample is final,
// whether counts are per-interval or running totals (temporality), and the basis of any cost figure.
const (
	UsageCoverageComplete = "complete"
	UsageCoveragePartial  = "partial"
	UsageCoverageUnknown  = "unknown"

	UsageFinalityFinal   = "final"
	UsageFinalityPartial = "partial"
	UsageFinalityUnknown = "unknown"

	UsageTemporalityDelta      = "delta"
	UsageTemporalityCumulative = "cumulative"

	UsageCostBasisHostEstimate = "host_reported_estimate"
	UsageCostBasisUnknown      = "unknown"
)

// UsageSample is one host-reported usage record. A nil metric means the host
// did not report a trustworthy value (unknown); a pointer to zero is a measured
// zero. Identifiers are SHA-256 digests of the bounded original identifiers.
type UsageSample struct {
	Host             string   `json:"host"`
	Version          string   `json:"version"`
	ScopeID          string   `json:"scope_id"`
	SessionID        string   `json:"session_id"`
	TurnID           string   `json:"turn_id"`
	MessageID        string   `json:"message_id"`
	Provider         string   `json:"provider"`
	Model            string   `json:"model"`
	Finality         string   `json:"finality"`
	Temporality      string   `json:"temporality"`
	Epoch            string   `json:"epoch"`
	CostBasis        string   `json:"cost_basis"`
	InputTokens      *int64   `json:"input_tokens"`
	OutputTokens     *int64   `json:"output_tokens"`
	CacheReadTokens  *int64   `json:"cache_read_tokens"`
	CacheWriteTokens *int64   `json:"cache_write_tokens"`
	CostUSD          *float64 `json:"cost_usd"`
}

// UsageReport carries the samples of one host export. It never contains a total:
// samples with different temporality must not be summed by the reader.
type UsageReport struct {
	Samples  []UsageSample `json:"samples"`
	Coverage string        `json:"coverage"`
	Warnings []string      `json:"warnings"`
}
