package issueopsremote

import (
	"fmt"
	"strings"
)

func ScoreIssueOpsRemoteCandidates(req ScoringRequest) (ScoringResult, error) {
	provider := strings.ToLower(strings.TrimSpace(firstNonEmpty(req.Provider, req.Issue.Provider)))
	if provider == "" {
		provider = "github"
	}
	if provider != "github" && provider != "gitlab" {
		return ScoringResult{OK: false, Provider: provider}, fmt.Errorf("unsupported issueops remote provider %q", provider)
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = defaultIssueOpsRemoteThreshold
	}
	if threshold > 1 {
		return ScoringResult{OK: false, Provider: provider}, fmt.Errorf("threshold must be between 0 and 1")
	}
	issueText := strings.TrimSpace(req.Issue.Title + "\n" + req.Issue.Body)
	if issueText == "" {
		return ScoringResult{OK: false, Provider: provider, Threshold: threshold}, fmt.Errorf("issue.title or issue.body is required in the scoring request")
	}
	issueTokens := issueOpsRemoteTokens(issueText)
	result := normalizeIssueOpsRemoteScoringResult(ScoringResult{OK: true, Provider: provider, Threshold: threshold})
	typeEvidence := issueOpsRemoteLabelEvidence(issueTokens)
	scoreIssueOpsRemoteIssueCandidates(&result, provider, threshold, issueTokens, req.IssueCandidates)
	scoreIssueOpsRemoteLabelCandidates(&result, provider, threshold, issueTokens, typeEvidence, req.LabelCandidates)
	sortIssueOpsRemoteScoredItems(result.SelectedRelatedIssues)
	sortIssueOpsRemoteScoredItems(result.RejectedRelatedIssues)
	sortIssueOpsRemoteScoredItems(result.SelectedLabels)
	sortIssueOpsRemoteScoredItems(result.RejectedLabels)
	result.ApplyInstructions = issueOpsRemoteApplyInstructions(provider, result.SelectedRelatedIssues, result.SelectedLabels)
	if len(req.IssueCandidates) > 0 && len(result.SelectedRelatedIssues) == 0 {
		result.Warnings = append(result.Warnings, "no related issue candidates met threshold")
	}
	if len(req.LabelCandidates) > 0 && len(result.SelectedLabels) == 0 {
		result.Warnings = append(result.Warnings, "no label candidates met threshold")
		result.ApplyInstructions = append(result.ApplyInstructions, "stop before remote artifact writes and choose an explicit manual label or rerun scoring with corrected candidates; do not create an unlabeled issue, pull request, or merge request")
	}
	return normalizeIssueOpsRemoteScoringResult(result), nil
}

func scoreIssueOpsRemoteIssueCandidates(result *ScoringResult, provider string, threshold float64, issueTokens map[string]bool, candidates []IssueCandidate) {
	for _, candidate := range candidates {
		item := scoreIssueOpsRemoteIssue(provider, threshold, issueTokens, candidate)
		if item.Selected {
			result.SelectedRelatedIssues = append(result.SelectedRelatedIssues, item)
		} else {
			result.RejectedRelatedIssues = append(result.RejectedRelatedIssues, item)
		}
	}
}

func scoreIssueOpsRemoteLabelCandidates(result *ScoringResult, provider string, threshold float64, issueTokens map[string]bool, typeEvidence issueOpsRemoteTypeEvidence, candidates []LabelCandidate) {
	for _, candidate := range candidates {
		item := scoreIssueOpsRemoteLabel(provider, threshold, issueTokens, typeEvidence, candidate)
		if item.Selected {
			result.SelectedLabels = append(result.SelectedLabels, item)
		} else {
			result.RejectedLabels = append(result.RejectedLabels, item)
		}
	}
}

func normalizeIssueOpsRemoteScoringResult(result ScoringResult) ScoringResult {
	if strings.TrimSpace(result.ExecutionClass) == "" {
		result.ExecutionClass = "background_join"
	}
	result.ReadOnly = true
	if strings.TrimSpace(result.JoinBefore) == "" {
		result.JoinBefore = "remote_artifact_write"
	}
	if result.SelectedRelatedIssues == nil {
		result.SelectedRelatedIssues = []ScoredItem{}
	}
	if result.RejectedRelatedIssues == nil {
		result.RejectedRelatedIssues = []ScoredItem{}
	}
	if result.SelectedLabels == nil {
		result.SelectedLabels = []ScoredItem{}
	}
	if result.RejectedLabels == nil {
		result.RejectedLabels = []ScoredItem{}
	}
	if result.ApplyInstructions == nil {
		result.ApplyInstructions = []string{}
	}
	if result.Warnings == nil {
		result.Warnings = []string{}
	}
	return result
}

func scoreIssueOpsRemoteIssue(provider string, threshold float64, issueTokens map[string]bool, candidate IssueCandidate) ScoredItem {
	score := scorePtr(candidate.Score)
	evidence := []string{}
	if candidate.Score != nil {
		evidence = append(evidence, "explicit score provided")
	} else {
		candidateText := strings.TrimSpace(candidate.Title + "\n" + candidate.Body + "\n" + strings.Join(candidate.Labels, " "))
		overlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(candidateText))
		titleOverlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(candidate.Title))
		labelOverlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(strings.Join(candidate.Labels, " ")))
		score = clampScore(0.55*overlap + 0.30*titleOverlap + 0.15*labelOverlap)
		evidence = append(evidence, fmt.Sprintf("token_overlap=%.2f", overlap), fmt.Sprintf("title_overlap=%.2f", titleOverlap), fmt.Sprintf("label_overlap=%.2f", labelOverlap))
	}
	selected := score >= threshold
	item := ScoredItem{
		ID:        strings.TrimSpace(candidate.ID),
		Title:     strings.TrimSpace(candidate.Title),
		URL:       strings.TrimSpace(candidate.URL),
		Score:     score,
		Threshold: threshold,
		Selected:  selected,
		Evidence:  evidence,
	}
	if selected {
		item.ApplyHint = issueOpsRemoteIssueApplyHint(provider, item)
	} else {
		item.RejectReason = "score below threshold"
	}
	return item
}

func scoreIssueOpsRemoteLabel(provider string, threshold float64, issueTokens map[string]bool, typeEvidence issueOpsRemoteTypeEvidence, candidate LabelCandidate) ScoredItem {
	score := scorePtr(candidate.Score)
	evidence := []string{}
	rejectReason := "score below threshold"
	if candidate.Score != nil {
		evidence = append(evidence, "explicit score provided")
	} else if typeScore, matched, ok := issueOpsRemoteLabelEvidenceScore(typeEvidence, candidate); ok {
		score = clampScore(typeScore)
		evidence = append(evidence, fmt.Sprintf("type_evidence=%d/%d", len(matched), typeEvidence.total), "matched_tokens="+strings.Join(matched, ","))
		if typeEvidence.total < issueOpsRemoteMinLabelEvidence {
			// A matched token next to score 0 read as a scoring bug (QA); say
			// that the scorer abstains on thin evidence by design.
			rejectReason = fmt.Sprintf("abstained: %d type-evidence token(s), scoring needs %d; choose the label manually", typeEvidence.total, issueOpsRemoteMinLabelEvidence)
		}
	} else {
		labelText := strings.TrimSpace(candidate.Name + "\n" + candidate.Description + "\n" + strings.Join(candidate.Aliases, " "))
		overlap := issueOpsRemoteOverlap(issueTokens, issueOpsRemoteTokens(labelText))
		score = clampScore(0.65 * overlap)
		evidence = append(evidence, fmt.Sprintf("token_overlap=%.2f", overlap), "type_heuristic=0.00")
	}
	selected := score >= threshold
	item := ScoredItem{
		Name:      strings.TrimSpace(candidate.Name),
		Score:     score,
		Threshold: threshold,
		Selected:  selected,
		Evidence:  evidence,
	}
	if selected {
		item.ApplyHint = issueOpsRemoteLabelApplyHint(provider, item.Name)
	} else {
		item.RejectReason = rejectReason
	}
	return item
}
