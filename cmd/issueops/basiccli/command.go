// Package basiccli implements the issueops docs, guard, inspect, doctor,
// preflight and trace subcommands: flag parsing and text/JSON rendering over
// application services that the composition root injects through Command and
// Doctor.
package basiccli

import (
	guardapp "issueops/internal/application/guard"
	preflightapp "issueops/internal/application/preflight"
	traceapp "issueops/internal/application/trace"
	auditcontract "issueops/internal/contract/audit"
	docscontract "issueops/internal/contract/docs"
	inspectcontract "issueops/internal/contract/inspect"
	issueopscontract "issueops/internal/contract/issueops"
)

type handoffObserver interface {
	ObserveManual(issueopscontract.HandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
}

// Command carries the injected services and settings shared by the basic CLI
// subcommands (docs index, harness inspection, preflight, guard, trace and
// handoff observation). The docs and preflight subcommands return a "not
// configured" error when their dependency is missing.
type Command struct {
	IssueOpsRoot   string
	DefaultTarget  string
	Version        string
	DocsIndex      func(root, version string, includeHistory bool) docscontract.IndexResult
	InspectHarness func(string, inspectcontract.Options) inspectcontract.Info
	Preflight      preflightapp.Service
	Guard          guardapp.Service
	Trace          traceapp.Service
	Handoff        handoffObserver
}
