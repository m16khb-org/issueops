package doctor

import (
	"time"

	contract "issueops/internal/contract/doctor"
)

// ProjectDocsObservation records which standard .issueops documents are
// missing from Directory.
type ProjectDocsObservation struct {
	Directory string
	Missing   []string
}

// RuntimeStateObservation records repo-local runtime state paths that should
// not be committed, plus the STATE.md path and text to scan for runtime/schema
// descriptions.
type RuntimeStateObservation struct {
	Paths                  []string
	DocumentPath, Document string
}

// LoopObservation counts the repo's active and exhausted loop contracts and
// carries warnings for loop records that could not be read; StateRoot is the
// directory they live in.
type LoopObservation struct {
	Active, Exhausted int
	Warnings          []string
	StateRoot         string
}

// PipeObservation is the measured system pipe buffer Capacity in bytes, or the
// Error that prevented measuring it.
type PipeObservation struct {
	Capacity int
	Error    error
}

// GatewayEndpoint is a configured loopback MCP server and the Error from its
// initialize probe, nil when it answered.
type GatewayEndpoint struct {
	Name, URL string
	Error     error
}

// GatewayFD is the open file descriptor Count of the gateway listening on
// Port; Available is false when the count could not be read.
type GatewayFD struct {
	Port, Count int
	Available   bool
}

// GatewayObservation gathers the loopback MCP gateway probes for the user's
// Home: the configured Endpoints, their file descriptor counts, and any error
// reading the MCP configuration.
type GatewayObservation struct {
	Home        string
	ConfigError error
	Endpoints   []GatewayEndpoint
	FDs         []GatewayFD
}

// NativeObservation records the user's Home and whether the Codex hooks file
// at HooksPath is missing.
type NativeObservation struct {
	Home, HooksPath string
	HooksMissing    bool
}

// BinaryObservation describes the prebuilt bin/issueops under Root: whether it
// was Found, when it was built, and the newest source modification time.
type BinaryObservation struct {
	Root, Path            string
	Found                 bool
	BuiltAt, LatestSource time.Time
}

// Observations is the complete input to Evaluate: the repository Root and one
// observation per doctor check area. Pipe and Gateways are nil when not
// measured.
type Observations struct {
	Root         string
	ProjectDocs  ProjectDocsObservation
	RuntimeState RuntimeStateObservation
	Loop         LoopObservation
	Pipe         *PipeObservation
	Gateways     *GatewayObservation
	Native       NativeObservation
	Binary       BinaryObservation
}

// Findings is the result of an evaluation: the checks run, the issues found,
// and the measured pipe capacity in bytes (zero when not measured).
type Findings struct {
	Checks            []contract.HarnessDoctorCheck
	Issues            []contract.HarnessDoctorIssue
	PipeCapacityBytes int
}

func (f *Findings) check(name string, healthy bool, summary string) {
	f.Checks = append(f.Checks, contract.HarnessDoctorCheck{Name: name, Healthy: healthy, Summary: summary})
}
func (f *Findings) issue(code, severity, summary, path string, fix *contract.HarnessDoctorFix) {
	f.Issues = append(f.Issues, contract.HarnessDoctorIssue{Code: code, Severity: severity, Summary: summary, Path: path, Fix: fix})
}
