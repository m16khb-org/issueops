package architecture

import (
	"slices"
	"strings"
	"testing"
)

// TestIssueOpsViewVerticalOwnsCycleViews는 view vertical이 네 계층 패키지를
// 모두 갖고, 그 어느 쪽도 retired adapter(internal/adapter/issueops와 하위),
// 리뷰 판정 domain(internal/domain/issueopsreview), cycle 관측 application
// (internal/application/issueopscycle)을 import하지 않음을 잠근다. stale 판정은
// next 포트로, 보존 행 읽기는 RetainedOutcome 포트로만 들어온다.
func TestIssueOpsViewVerticalOwnsCycleViews(t *testing.T) {
	requiredPackages := []string{
		"internal/contract/issueopsview",
		"internal/domain/issueopsview",
		"internal/application/issueopsview",
		"internal/adapter/inbound/issueopsview",
	}
	productionPackages := loadProductionPackages(t)
	for _, required := range requiredPackages {
		if !slices.Contains(productionPackages, required) {
			t.Errorf("missing issueops view package %s", required)
		}
	}

	forbidden := []string{
		"internal/adapter/issueops",
		"internal/domain/issueopsreview",
		"internal/application/issueopscycle",
	}
	for _, edge := range loadProductionEdges(t) {
		if !strings.Contains(edge.importer, "issueopsview") {
			continue
		}
		for _, banned := range forbidden {
			if edge.imported == banned || strings.HasPrefix(edge.imported, banned+"/") {
				t.Errorf("issueops view vertical imports %s: %s", banned, formatEdge(edge))
			}
		}
	}
}
