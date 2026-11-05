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

// ErrMissingArchitectureReviewEvidence 변수는 승인된 아키텍처 리뷰의 verification에
// 아키텍처 검토를 가리키는 항목이 없을 때 FinalizeArchitectureReview가 돌려주는 오류다.
var ErrMissingArchitectureReviewEvidence = errors.New("approved architecture review requires architecture review evidence")

// PrepareArchitectureReview 함수는 아키텍처 리뷰 요청을 검증하고 정리한 기록을 만든다.
// 문제 요약, 제안 구조, verification, 올바른 reviewer_context가 필수이며 승인 기록에
// open_questions가 남아 있으면 오류다. 자유 서술 필드는 비밀 값이 가려진다.
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
		ReviewerHost:    strings.ToLower(strings.TrimSpace(req.ReviewerHost)),
		ReviewerModel:   strings.TrimSpace(req.ReviewerModel),
		ReviewerEffort:  strings.TrimSpace(req.ReviewerEffort),
	}, nil
}

// FinalizeArchitectureReview 함수는 리뷰에 reviewedAt을 찍어 확정한다. 승인 기록은
// refactor_plan, alternatives, risks와 아키텍처 검토 근거가 모두 있어야 하며 빠지면 오류다.
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
	if review.Approved && !hasArchitectureReviewEvidence(review.Verification) {
		return reviewcontract.ArchitectureReview{}, ErrMissingArchitectureReviewEvidence
	}
	review.ReviewedAt = reviewedAt
	return review, nil
}

func hasArchitectureReviewEvidence(values []string) bool {
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

// NonPlanPrepMissing 함수는 missing 목록에서 plan_prep_ 접두사 항목을 제외하고
// 나머지를 입력 순서대로 돌려준다.
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

// ApplyArchitectureReview 함수는 이전 기록을 history 끝에 보존한 채 새 기록을 돌려준다.
func ApplyArchitectureReview(previous *reviewcontract.ArchitectureReview, next reviewcontract.ArchitectureReview) reviewcontract.ArchitectureReview {
	if previous == nil {
		return next
	}
	next.History = append(append([]reviewcontract.ArchitectureReviewRound{}, previous.History...), reviewcontract.ArchitectureReviewRound{
		Approved: previous.Approved, Verification: previous.Verification, OpenQuestions: previous.OpenQuestions,
		ReviewerContext: previous.ReviewerContext, ReviewedArchitectureDigest: previous.ReviewedArchitectureDigest,
		ReviewerHost: previous.ReviewerHost, ReviewerModel: previous.ReviewerModel, ReviewerEffort: previous.ReviewerEffort,
		ReviewedAt: previous.ReviewedAt,
	})
	return next
}

// ArchitectureDocumentDigest 함수는 아키텍처 문서가 리뷰를 기록할 수 있는 상태인지
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
