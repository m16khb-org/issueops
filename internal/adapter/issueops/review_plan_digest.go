package issueops

import (
	"fmt"
	"strings"

	"issueops/internal/contract/issueops"
)

// issueOpsLinkedPlanDigest는 링크된 플랜 파일의 sha256이다. owner preflight와
// 같은 resolver(readLinkedPlanIdentity)를 써서 한 플랜에 digest가 둘이 되지 않게 한다.
func issueOpsLinkedPlanDigest(record issueops.IssueOpsRecord) (string, error) {
	identity, err := readLinkedPlanIdentity(record)
	if err != nil {
		return "", err
	}
	return identity.Digest, nil
}

// issueOpsReviewedPlanDigest는 devil's-advocate 판정을 묶을 플랜 digest다. 링크된
// 플랜 파일이 우선이고, 파일 없이 staged plan artifact만 있는 사이클(fresh staged
// plan)은 그 artifact에 묶는다. 둘 다 없으면 검토할 플랜이 없으므로 기록을 거부한다.
func issueOpsReviewedPlanDigest(stateRoot string, record issueops.IssueOpsRecord) (string, error) {
	if strings.TrimSpace(record.PlanPath) != "" {
		return issueOpsLinkedPlanDigest(record)
	}
	staged, err := readStagedArtifacts(stateRoot, record.ID)
	if err == nil {
		if plan, ok := staged["plan"]; ok && strings.TrimSpace(plan) != "" {
			return digestExecutionOwnerBytes([]byte(plan)), nil
		}
	}
	return "", fmt.Errorf("link the plan (issueops link-plan) or stage it (issueops artifact stage --name plan) before recording the devil's-advocate review")
}
