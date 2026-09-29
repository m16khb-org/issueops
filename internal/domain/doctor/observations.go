package doctor

import (
	"time"

	contract "issueops/internal/contract/doctor"
)

type ProjectDocsObservation struct {
	Directory string
	Missing   []string
}
type RuntimeStateObservation struct {
	Paths                  []string
	DocumentPath, Document string
}
type LoopObservation struct {
	Active, Exhausted int
	Warnings          []string
	StateRoot         string
}
type PipeObservation struct {
	Capacity int
	Error    error
}
type GatewayEndpoint struct {
	Name, URL string
	Error     error
}
type GatewayFD struct {
	Port, Count int
	Available   bool
}
type GatewayObservation struct {
	Home        string
	ConfigError error
	Endpoints   []GatewayEndpoint
	FDs         []GatewayFD
}
type NativeObservation struct {
	Home, HooksPath string
	HooksMissing    bool
}
type BinaryObservation struct {
	Root, Path            string
	Found                 bool
	BuiltAt, LatestSource time.Time
}

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
