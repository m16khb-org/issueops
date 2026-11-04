// Package apidoc is the application layer for API documentation gates: a
// deterministic static check of NestJS controllers and DTOs followed by a
// host-agent review of the documentation diff.
package apidoc

import (
	"fmt"
	contract "issueops/internal/contract/apidoc"
)

// Service runs the static gate before requesting a host-agent review.
type Service struct {
	Static   StaticService
	Reviewer ReviewService
}

// Check runs the static check and, only when it passes, the host-agent review.
// A failed static check short-circuits with a skipped passing review and an
// error; otherwise the review's error is returned with the combined result.
func (s Service) Check(staticOptions StaticOptions, reviewOptions ReviewOptions) (contract.CheckResult, error) {
	staticResult, staticErr := s.Static.Check(staticOptions)
	if staticErr != nil || !staticResult.OK {
		return contract.CheckResult{OK: false, Static: staticResult, Review: contract.ReviewResult{OK: true, Verdict: "pass", Summary: "Agent review skipped because static API documentation check failed.", Findings: []contract.ReviewFinding{}, Files: staticResult.Files, Skipped: true, Reason: "static_check_failed"}, Reason: "static_check_failed"}, fmt.Errorf("api documentation static check failed")
	}
	reviewResult, reviewErr := s.Reviewer.Review(reviewOptions)
	return contract.CheckResult{OK: reviewErr == nil && reviewResult.OK, Static: staticResult, Review: reviewResult}, reviewErr
}
