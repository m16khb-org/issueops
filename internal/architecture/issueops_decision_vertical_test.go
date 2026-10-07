package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsDecisionVerticalOwnsDecisionRecording(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsdecision",
		"internal/domain/issueopsdecision",
		"internal/application/issueopsdecision",
		"internal/adapter/inbound/issueopsdecision",
		"internal/adapter/outbound/issueopsdecision",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops decision package %s", required)
		}
	}

	retiredPath := filepath.Join(
		findRepoRoot(t),
		"internal",
		"adapter",
		"issueops",
		"issueops_decision.go",
	)
	if _, err := os.Stat(retiredPath); err == nil {
		t.Errorf("retired decision implementation must be deleted: %s", retiredPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect retired decision implementation: %v", err)
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsdecision") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops decision vertical imports the retired adapter package: %s", formatEdge(edge))
		}
	}
}
