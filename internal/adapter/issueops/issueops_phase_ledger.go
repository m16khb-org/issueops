package issueops

import (
	"strings"

	"issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

func issueOpsReadinessFrom(record issueops.IssueOpsRecord, missing []string) issueops.IssueOpsReadiness {
	return issueops.IssueOpsReadiness{
		OK:           true,
		Ready:        len(missing) == 0,
		Missing:      stringlist.UniqueSorted(missing),
		IssueURL:     record.IssueURL,
		PlanPath:     record.PlanPath,
		WorktreePath: record.WorktreePath,
		Branch:       record.Branch,
	}
}

// IssueOpsProblemReadiness는 problem phase 완료 여부를 보고한다. problem 완료는
// intent contract만 요구하도록 의도적으로 최소화한다. remote issue나 branch가
// 생기기 전의 자유로운 problem -> grill 전이와 초기 탐색을 보존하기 위해서다.
// issue_url/branch는 grill artifact다.
func IssueOpsProblemReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsIntentMissing(record))
}

// IssueOpsGrillReadiness는 grill phase 완료 여부를 보고한다. 필요한 것은
// issue_url + branch + plan_prep(적용 시) + split_decision + domain_review다.
// 이는 create-issue-after-grill workflow와 현재 plan-entry gate에 맞춰 plan
// 진입을 막는다.
func IssueOpsGrillReadiness(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.IssueURL) == "" {
		missing = append(missing, "issue_url")
	}
	if strings.TrimSpace(record.Branch) == "" {
		missing = append(missing, "branch")
	}
	if planPrepGateApplies(record) {
		missing = append(missing, planPrepMissing(record.PlanPrep)...)
	}
	missing = append(missing, issueopsdomain.SplitDecisionMissing(record)...)
	missing = append(missing, issueopsdomain.DomainReviewMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsPlanCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// plan 완료(branch_prepare + worktree + plan + design_review)는 현재
	// "compatibility-review 진입 준비 완료" gate와 정확히 같다.
	return IssueOpsCompatibilityReviewReadiness(record)
}

func issueOpsCompatibilityReviewCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	return issueOpsReadinessFrom(record, issueOpsCompatibilityReviewMissing(record))
}

func issueOpsImplementCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// implement 완료는 implement-entry readiness에 종료 artifact인
	// implementation_changes를 더한다. 이는 현재 ai-slop-clean entry gate와 같다.
	return IssueOpsAISlopCleanReadiness(record)
}

func issueOpsAISlopCleanCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if strings.TrimSpace(record.AISlopCleanAt) == "" {
		missing = append(missing, "ai_slop_clean_at")
	}
	if strings.TrimSpace(record.AISlopCleanHead) == "" {
		missing = append(missing, "ai_slop_clean_head")
	}
	if strings.TrimSpace(record.AISlopCleanFingerprint) == "" {
		missing = append(missing, "ai_slop_clean_fingerprint")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanCategories)) == 0 {
		missing = append(missing, "cleanup_evidence")
	}
	if len(cleanIssueOpsTextValues(record.AISlopCleanVerification)) == 0 {
		missing = append(missing, "verification_evidence")
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsFeedbackCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Classification) == "" {
			missing = append(missing, "feedback_classification")
			break
		}
	}
	if issueOpsHasUnresolvedContractFeedback(record) {
		missing = append(missing, "contract_feedback_issue_update")
	}
	for _, item := range record.Feedback {
		if strings.TrimSpace(item.Resolution) == "" {
			missing = append(missing, "feedback_resolution")
			break
		}
	}
	return issueOpsReadinessFrom(record, missing)
}

func issueOpsPRCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	// 완료/파생에는 git fetch를 하지 않는 non-strict readiness를 사용한다. network
	// 부수효과 없이 status 표시용 ledger를 파생하기 위해서다. 실제 pr-phase entry
	// gate는 계속 IssueOpsStrictPRReadiness를 사용한다.
	ready := IssueOpsPRReadiness(record)
	missing := append([]string{}, ready.Missing...)
	if record.RemoteArtifact == nil || strings.TrimSpace(record.RemoteArtifact.URL) == "" {
		missing = append(missing, "remote_artifact")
	}
	missing = append(missing, issueopsdomain.TargetBranchMatchMissing(record)...)
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func issueOpsDoneCompletion(record issueops.IssueOpsRecord) issueops.IssueOpsReadiness {
	missing := []string{}
	if issueOpsPhaseRank(record.Phase) < issueOpsPhaseRank(IssueOpsPhasePR) {
		missing = append(missing, "prior_phase_pr")
	}
	missing = append(missing, issueOpsRemoteArtifactMissing(record)...)
	return issueOpsReadinessFrom(record, missing)
}

// IssueOpsPhaseCompletion은 기존 source-of-truth 필드에서 phase 완료 여부를
// 계산해 ready/artifacts(missing)를 반환한다. 기존 readiness 함수를 색인할 뿐,
// 스스로 source of truth가 되지는 않는다.
func IssueOpsPhaseCompletion(record issueops.IssueOpsRecord, phase issueops.IssueOpsPhase) issueops.IssueOpsReadiness {
	switch phase {
	case IssueOpsPhaseProblem:
		return IssueOpsProblemReadiness(record)
	case IssueOpsPhaseGrill:
		return IssueOpsGrillReadiness(record)
	case IssueOpsPhasePlan:
		return issueOpsPlanCompletion(record)
	case IssueOpsPhaseCompatibilityReview:
		return issueOpsCompatibilityReviewCompletion(record)
	case IssueOpsPhaseImplement:
		return issueOpsImplementCompletion(record)
	case IssueOpsPhaseAISlopClean:
		return issueOpsAISlopCleanCompletion(record)
	case IssueOpsPhaseFeedback:
		return issueOpsFeedbackCompletion(record)
	case IssueOpsPhasePR:
		return issueOpsPRCompletion(record)
	case IssueOpsPhaseDone:
		return issueOpsDoneCompletion(record)
	default:
		return issueops.IssueOpsReadiness{OK: true, Ready: false, Missing: []string{"unknown_phase"}}
	}
}
