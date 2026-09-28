package issueopsreview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func HasLinkedReviewPlan(path string) bool { return strings.TrimSpace(path) != "" }

func StagedReviewPlanDigest(body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("link the plan (issueops link-plan) or stage it (issueops artifact stage --name plan) before recording the devil's-advocate review")
	}
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:]), nil
}
