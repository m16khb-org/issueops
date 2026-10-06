package issueopsapp

import (
	outbound "issueops/internal/adapter/outbound/quality"
	app "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	qualitycatalogcontract "issueops/internal/contract/qualitycatalog"
	catalog "issueops/internal/domain/qualitycatalog"
	"os"
	"path/filepath"
	"testing"
)

func writeValidZeroAudit(t *testing.T, root string) {
	t.Helper()
	path := filepath.Join(root, ".issueops", "PROJECT_AUDIT.md")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("## Summary Matrix\n### Open\n_None. All triaged P1/P2 items are resolved or accepted-with-rationale below._\n"), 0600); err != nil {
		t.Fatal(err)
	}
}

func inspectQualityForTest(root string, deps app.InspectDeps) contract.InspectResult {
	if deps.BranchFunctions == nil {
		deps.BranchFunctions = outbound.CollectBranchFunctions
	}
	if deps.AuditItems == nil {
		deps.AuditItems = outbound.CollectAuditItems
	}
	if deps.Candidates == nil {
		deps.Candidates = func(string) []qualitycatalogcontract.Candidate { return catalog.Candidates() }
	}
	return app.Inspect(root, deps)
}
