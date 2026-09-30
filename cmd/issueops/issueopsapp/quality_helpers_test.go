package issueopsapp

import (
	outbound "issueops/internal/adapter/outbound/quality"
	app "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	catalog "issueops/internal/domain/qualitycatalog"
)

func inspectQualityForTest(root string, deps app.InspectDeps) contract.InspectResult {
	if deps.BranchFunctions == nil {
		deps.BranchFunctions = outbound.CollectBranchFunctions
	}
	if deps.AuditItems == nil {
		deps.AuditItems = outbound.CollectAuditItems
	}
	if deps.Candidates == nil {
		deps.Candidates = func(string) []catalog.Candidate { return catalog.Candidates() }
	}
	return app.Inspect(root, deps)
}
