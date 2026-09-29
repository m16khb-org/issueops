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

type HandoffObserver interface {
	ObserveManual(issueopscontract.IssueOpsHandoffDeliveryObservation) (auditcontract.HandoffDeliveryAuditRecord, error)
}

type Command struct {
	IssueOpsRoot   string
	DefaultTarget  string
	Version        string
	DocsIndex      func(string, string) docscontract.DocsIndexResult
	InspectHarness func(string) inspectcontract.InspectInfo
	Preflight      preflightapp.Service
	Guard          guardapp.Service
	Trace          traceapp.Service
	Handoff        HandoffObserver
}
