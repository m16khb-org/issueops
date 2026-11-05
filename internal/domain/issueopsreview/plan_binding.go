package issueopsreview

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// HasLinkedReviewPlan reports whether path names a linked plan file.
func HasLinkedReviewPlan(path string) bool { return strings.TrimSpace(path) != "" }

// StagedReviewPlanDigest returns the hex sha256 of the staged plan body that a review
// binds to. An empty body is an error that tells the caller to link or stage the plan.
func StagedReviewPlanDigest(body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("link the plan (issueops link-plan) or stage it (issueops artifact stage --name plan) before recording the devil's-advocate review")
	}
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:]), nil
}
