package issueopsreview

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/policy"
)

var ErrMissingArchitectureReviewEvidence = errors.New("approved architecture review requires architecture review evidence")

func PrepareArchitectureReview(req reviewcontract.ArchitectureReviewRequest) (reviewcontract.ArchitectureReview, error) {
	problemSummary := strings.TrimSpace(req.ProblemSummary)
	if problemSummary == "" {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("problem_summary is required (--problem-summary TEXT)")
	}
	proposedArchitecture := strings.TrimSpace(req.ProposedArchitecture)
	if proposedArchitecture == "" {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("proposed_architecture is required (--proposed-architecture TEXT)")
	}
	verification := cleanArchitectureValues(req.Verification)
	if len(verification) == 0 {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("verification is required (--verification TEXT, repeatable)")
	}
	openQuestions := cleanArchitectureValues(req.OpenQuestions)
	if req.Approved && len(openQuestions) > 0 {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("approved architecture review must not have open_questions")
	}
	reviewerContext := strings.ToLower(strings.TrimSpace(req.ReviewerContext))
	if reviewerContext != "subagent" && reviewerContext != "inline" {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("reviewer_context must be subagent or inline (--reviewer-context subagent|inline)")
	}
	return reviewcontract.ArchitectureReview{
		ProblemSummary:       policy.RedactFreeform(problemSummary),
		ProposedArchitecture: policy.RedactFreeform(proposedArchitecture),
		RefactorPlan:         policy.RedactFreeform(strings.TrimSpace(req.RefactorPlan)),
		Alternatives:         cleanArchitectureValues(req.Alternatives), Risks: cleanArchitectureValues(req.Risks),
		Verification: verification, OpenQuestions: openQuestions, Approved: req.Approved,
		ReviewerContext: reviewerContext,
	}, nil
}

func FinalizeArchitectureReview(review reviewcontract.ArchitectureReview, reviewedAt string) (reviewcontract.ArchitectureReview, error) {
	if review.Approved && review.RefactorPlan == "" {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("approved architecture review requires refactor_plan")
	}
	if review.Approved && len(review.Alternatives) == 0 {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("approved architecture review requires alternatives")
	}
	if review.Approved && len(review.Risks) == 0 {
		return reviewcontract.ArchitectureReview{}, fmt.Errorf("approved architecture review requires risks")
	}
	if review.Approved && !HasArchitectureReviewEvidence(review.Verification) {
		return reviewcontract.ArchitectureReview{}, ErrMissingArchitectureReviewEvidence
	}
	review.ReviewedAt = reviewedAt
	return review, nil
}

func HasArchitectureReviewEvidence(values []string) bool {
	for _, value := range values {
		text := strings.ToLower(strings.TrimSpace(value))
		if text == "" {
			continue
		}
		if strings.Contains(text, "architecture") && (strings.Contains(text, "review") || strings.Contains(text, "audit") || strings.Contains(text, "evaluat")) {
			return true
		}
		if strings.Contains(text, "아키텍처") && (strings.Contains(text, "검수") || strings.Contains(text, "검토")) {
			return true
		}
	}
	return false
}

func NonPlanPrepMissing(missing []string) []string {
	out := []string{}
	for _, item := range missing {
		if !strings.HasPrefix(item, "plan_prep_") {
			out = append(out, item)
		}
	}
	return out
}

func cleanArchitectureValues(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "\x00") || seen[value] {
			continue
		}
		value = policy.RedactFreeform(value)
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

// ApplyArchitectureReview는 이전 기록을 history 끝에 보존한 채 새 기록을 돌려준다.
func ApplyArchitectureReview(previous *reviewcontract.ArchitectureReview, next reviewcontract.ArchitectureReview) reviewcontract.ArchitectureReview {
	if previous == nil {
		return next
	}
	next.History = append(append([]reviewcontract.ArchitectureReviewRound{}, previous.History...), reviewcontract.ArchitectureReviewRound{
		Approved: previous.Approved, Verification: previous.Verification, OpenQuestions: previous.OpenQuestions,
		ReviewerContext: previous.ReviewerContext, ReviewedArchitectureDigest: previous.ReviewedArchitectureDigest,
		ReviewedAt: previous.ReviewedAt,
	})
	return next
}

// ArchitectureDocumentDigest는 아키텍처 문서가 리뷰를 기록할 수 있는 상태인지
// 확인하고 승인에 묶을 sha256을 돌려준다. 문서가 없거나 필수 절이 비어 있으면 거부한다.
func ArchitectureDocumentDigest(body string) (string, error) {
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("architecture review requires a staged architecture artifact")
	}
	if missing := issueopsdomain.MissingArchitectureSections(body); len(missing) > 0 {
		return "", fmt.Errorf("architecture is missing required sections: %s", strings.Join(missing, ", "))
	}
	digest := sha256.Sum256([]byte(body))
	return hex.EncodeToString(digest[:]), nil
}
