package trace

import tracecontract "issueops/internal/contract/trace"

type Evidence struct{ Cause, Code, Source string }
type Cluster struct {
	Step  string
	Count int
}
type Summary struct {
	FailedSteps              int
	FailedStep, FailureClass string
	Clusters                 []Cluster
	RerunCommands            []string
	Evidence                 []Evidence
	Cause                    string
}
type Upkeep struct {
	Kind, Summary string
	TargetDocs    []string
}
type Document struct {
	Summary              Summary
	HasSummary, HasGuard bool
	TopFailureClass      string
	TopFailedSteps       int
	GuardRules           []string
	Upkeep               Upkeep
	Event, Step          string
	OK                   bool
}
type Input struct {
	Document   *Document
	Lines      []Document
	Incomplete bool
	Warnings   []string
	Usage      *tracecontract.UsageReport
}
type Finding struct {
	FailureClass, FailureCause                                       string
	FailureCauseEvidence                                             []Evidence
	RecurringPattern, ProposedKnob, OverfitRisk, VerificationCommand string
}
type Analysis struct {
	Findings        []Finding
	Types, Warnings []string
	Incomplete      bool
}
