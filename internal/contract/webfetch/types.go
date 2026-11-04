// Package webfetch defines the request, result, response validation and benchmark DTOs of the resilient public web fetch, together with its category and stop-reason constants.
package webfetch

import (
	"time"
)

// CategoryStrongOK through CategoryUnknown classify a fetched response in Result.Category; only CategoryStrongOK counts as a trustworthy full-content success.
const (
	CategoryStrongOK     = "strong_ok"
	CategoryWeakOK       = "weak_ok"
	CategorySuspectOK    = "suspect_ok"
	CategoryChallenge    = "challenge"
	CategoryBlocked      = "blocked"
	CategoryRateLimited  = "rate_limited"
	CategoryAuthRequired = "auth_required"
	CategoryPaywalled    = "paywalled"
	CategoryNotFound     = "not_found"
	CategoryUnknown      = "unknown"
)

// StopReasonAccepted through StopReasonError are the Result.StopReason values explaining why the fetch route search ended.
const (
	StopReasonAccepted       = "accepted"
	StopReasonSafetyRejected = "safety_rejected"
	StopReasonUnsafeRedirect = "unsafe_redirect"
	StopReasonRedirectLoop   = "redirect_loop"
	StopReasonGridExhausted  = "grid_exhausted"
	StopReasonRateLimited    = "rate_limited"
	StopReasonNotFound       = "not_found"
	StopReasonError          = "error"
)

// Request describes one fetch: the URL, an overall Timeout, the MaxChars limit on returned content, and whether private-network targets are allowed.
type Request struct {
	URL                 string
	Timeout             time.Duration
	MaxChars            int
	AllowPrivateNetwork bool
}

// Result is the outcome of a fetch: the final URL, response Category, StopReason, extracted Content and metadata, plus the routes that were attempted and left untried. DurationMS is in milliseconds.
type Result struct {
	OK              bool           `json:"ok"`
	URL             string         `json:"url"`
	FinalURL        string         `json:"final_url"`
	Category        string         `json:"category"`
	StopReason      string         `json:"stop_reason"`
	GridExhausted   bool           `json:"grid_exhausted"`
	AttemptedRoutes []RouteRecord  `json:"attempted_routes"`
	UntriedRoutes   []RouteRecord  `json:"untried_routes"`
	Content         string         `json:"content"`
	Metadata        map[string]any `json:"metadata"`
	Warnings        []string       `json:"warnings"`
	RetrievedAt     string         `json:"retrieved_at"`
	DurationMS      int64          `json:"duration_ms"`
}

// RouteRecord is the status of one fetch route and, when it did not succeed, the reason.
type RouteRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ResponseValidationInput is the HTTP response (status, headers, body, URL) handed to response validation.
type ResponseValidationInput struct {
	StatusCode int
	Header     map[string][]string
	Body       []byte
	URL        string
}

// ResponseValidation is the verdict on one HTTP response: its Category and StopReason, whether it is a Strong success, and the extracted Content, Metadata and Warnings.
type ResponseValidation struct {
	Category   string
	StopReason string
	Strong     bool
	Content    string
	Metadata   map[string]any
	Warnings   []string
}

// BenchmarkRequest configures a benchmark run over Fixtures. Live runs hit the network only when Live and LiveOptIn are both set, and CompareCommand names the baseline to compare against.
type BenchmarkRequest struct {
	Fixtures            []BenchmarkFixture
	Timeout             time.Duration
	Live                bool
	LiveOptIn           bool
	CompareCommand      string
	AllowPrivateNetwork bool
}

// BenchmarkFixture is one recorded response (status, headers, body) with the content snippets expected from it and flags for unsafe redirect and redirect loop cases.
type BenchmarkFixture struct {
	ID             string            `json:"id"`
	URL            string            `json:"url,omitempty"`
	StatusCode     int               `json:"status_code"`
	Headers        map[string]string `json:"headers"`
	Body           string            `json:"body"`
	Expected       []string          `json:"expected"`
	MinBodyChars   int               `json:"min_body_chars"`
	UnsafeRedirect bool              `json:"unsafe_redirect"`
	RedirectLoop   bool              `json:"redirect_loop"`
}

// BenchmarkResult is the benchmark outcome: the overall Score, hard failures, safety pass rate, the count of responses falsely accepted as strong, per-fixture runs, per-dimension scores, and the live parity evaluation.
type BenchmarkResult struct {
	OK                  bool                  `json:"ok"`
	Score               float64               `json:"score"`
	FixtureCount        int                   `json:"fixture_count"`
	HardFailures        []string              `json:"hard_failures,omitempty"`
	SafetyPassRate      float64               `json:"safety_pass_rate"`
	FalseStrongOK       int                   `json:"false_strong_ok"`
	FixtureResults      []BenchmarkFixtureRun `json:"fixture_results"`
	LiveParityEvaluated bool                  `json:"live_parity_evaluated"`
	LiveParityStatus    string                `json:"live_parity_status"`
	LiveParityReport    LiveParityReport      `json:"live_parity_report"`
	DimensionScores     map[string]float64    `json:"dimension_scores"`
}

// BenchmarkFixtureRun is the result of running one fixture, with its category, stop reason, failure message and latency in milliseconds.
type BenchmarkFixtureRun struct {
	ID         string `json:"id"`
	OK         bool   `json:"ok"`
	Category   string `json:"category"`
	StopReason string `json:"stop_reason"`
	Failure    string `json:"failure,omitempty"`
	LatencyMS  int64  `json:"latency_ms,omitempty"`
}

// LiveParityReport compares a live run against a baseline: success rate, category agreement, route count, p50 and p95 latencies in milliseconds, safety failures and false strong results.
type LiveParityReport struct {
	SuccessRate          float64  `json:"success_rate"`
	CategoryAgreement    float64  `json:"category_agreement"`
	RouteCount           int      `json:"route_count"`
	LatencyP50MS         int64    `json:"latency_p50_ms"`
	LatencyP95MS         int64    `json:"latency_p95_ms"`
	SafetyFailures       int      `json:"safety_failures"`
	FalseStrongOK        int      `json:"false_strong_ok"`
	BaselineAvailable    bool     `json:"baseline_available"`
	BaselineSuccessRate  float64  `json:"baseline_success_rate,omitempty"`
	BaselineLatencyP50MS int64    `json:"baseline_latency_p50_ms,omitempty"`
	BaselineLatencyP95MS int64    `json:"baseline_latency_p95_ms,omitempty"`
	Warnings             []string `json:"warnings"`
}
