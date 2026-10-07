package architecture

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestIssueOpsRetentionVerticalOwnsPruning(t *testing.T) {
	root := findRepoRoot(t)
	requiredPackages := []string{
		"internal/contract/issueopsretention",
		"internal/domain/issueopsretention",
		"internal/application/issueopsretention",
		"internal/adapter/inbound/issueopsretention",
		"internal/adapter/outbound/issueopsretention",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops retention package %s", required)
		}
	}

	retiredFile := filepath.Join(root, "internal", "adapter", "issueops", "issueops_prune.go")
	if _, err := os.Stat(retiredFile); err == nil {
		t.Errorf("retired prune implementation must be deleted: %s", retiredFile)
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect retired prune implementation: %v", err)
	}

	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsretention") {
			continue
		}
		if edge.imported == "internal/adapter/issueops" ||
			strings.HasPrefix(edge.imported, "internal/adapter/issueops/") {
			t.Errorf("issueops retention vertical imports the retired adapter package: %s", formatEdge(edge))
		}
	}
}
