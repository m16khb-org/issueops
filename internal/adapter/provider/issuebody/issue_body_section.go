// Package issuebody renders and idempotently splices delimited managed
// sections into a remote issue body, shared by the github and gitlab adapters.
package issuebody

import (
	"fmt"
	"strings"

	"issueops/internal/domain/artifactreadability"
	"issueops/internal/port"
)

// Managed section kinds. Each kind owns one delimited block; merging one kind
// never touches the other kind's block or any content outside the delimiters.
const (
	SectionDevilsAdvocate = port.IssueBodySectionDevilsAdvocate
	SectionCompletion     = port.IssueBodySectionCompletion
)

const (
	devilsAdvocateStartMarker = "<!-- issueops:devils-advocate:start -->"
	devilsAdvocateEndMarker   = "<!-- issueops:devils-advocate:end -->"
	// CompletionStartMarker is exported so cleanup readiness can readback-check
	// that the completion section was reflected before destructive cleanup.
	CompletionStartMarker = port.IssueBodyCompletionStartMarker
	completionEndMarker   = "<!-- issueops:completion:end -->"
)

// SectionMarkers resolves the delimiters for a managed section kind.
func SectionMarkers(section string) (start, end string, err error) {
	switch section {
	case SectionDevilsAdvocate:
		return devilsAdvocateStartMarker, devilsAdvocateEndMarker, nil
	case SectionCompletion:
		return CompletionStartMarker, completionEndMarker, nil
	}
	return "", "", fmt.Errorf("unsupported issue body section %q (want %s|%s)", section, SectionDevilsAdvocate, SectionCompletion)
}

// planReviewVerdictLabels are the words a reader sees for each verdict.
var planReviewVerdictLabels = map[string]string{
	"pass":   "통과",
	"revise": "수정 요청",
	"stop":   "중단",
}

// RenderDevilsAdvocateSection builds the delimited plan-review region: one
// flow line over every round ("1차 수정 요청(지적 3건) → 계획 수정 → 2차
// 통과"), and the stop reasons when the current verdict is a stop. Finding
// text for other verdicts stays in the record and in plan-review.md. The
// delimiters let MergeManagedSection replace the block in place on re-runs.
func RenderDevilsAdvocateSection(req port.IssueProviderUpdateIssueBodySectionRequest) string {
	var b strings.Builder
	b.WriteString(devilsAdvocateStartMarker + "\n## 계획 검토\n\n")
	rounds := req.Rounds
	if len(rounds) == 0 {
		rounds = []port.IssueProviderPlanReviewRound{{Verdict: req.Verdict, Findings: len(req.Findings)}}
	}
	steps := make([]string, 0, 2*len(rounds))
	for i, round := range rounds {
		if i > 0 {
			steps = append(steps, "계획 수정")
		}
		label := planReviewVerdictLabels[round.Verdict]
		if label == "" {
			label = round.Verdict
		}
		step := fmt.Sprintf("%d차 %s", i+1, label)
		if round.Findings > 0 {
			step += fmt.Sprintf("(지적 %d건)", round.Findings)
		}
		steps = append(steps, step)
	}
	b.WriteString("계획 검토: " + strings.Join(steps, " → ") + "\n")
	if req.Verdict == "stop" {
		b.WriteString("\n중단 이유:\n")
		for _, f := range req.Findings {
			if f = strings.TrimSpace(f); f != "" {
				fmt.Fprintf(&b, "- %s\n", artifactreadability.MaskHarnessValues(f))
			}
		}
	}
	b.WriteString(devilsAdvocateEndMarker)
	return b.String()
}

// RenderCompletionSection builds the delimited progress-report region: the
// markers, a "## 진행 결과" heading, and the written result. Nothing is
// truncated; a result over the body budget fails in RenderSection instead.
func RenderCompletionSection(c port.IssueProviderCompletionSection) string {
	return CompletionStartMarker + "\n## 진행 결과\n\n" + strings.TrimSpace(c.ResultBody) + "\n" + completionEndMarker
}

// SectionBudget은 병합 결과가 provider 본문 한도를 지키도록 섹션에 배정
// 가능한 바이트 예산을 계산한다: 한도 - (기존 본문 길이 - 교체될 기존 블록
// 길이). 렌더·절단을 섹션 단독 길이에만 적용하면 병합 결과가 한도를 넘을
// 수 있다(C3-F1).
func SectionBudget(body string, limit int, startMarker, endMarker string) int {
	if limit <= 0 {
		return 0
	}
	existing := 0
	if s := strings.Index(body, startMarker); s >= 0 {
		if e := strings.Index(body, endMarker); e > s {
			existing = e + len(endMarker) - s
		}
	}
	budget := limit - (len(body) - existing)
	// 0 이하는 전부 강제 절단 경로로 보낸다 — 하류가 limit <= 0을 "한도
	// 없음"으로 해석하므로 0을 그대로 흘리면 경계에서 초과 병합이 새어 나간다.
	if budget <= 0 {
		return 1
	}
	return budget
}

// MergeManagedSection replaces the delimited managed section in body with
// section, or appends it when absent. It never touches content outside the
// delimiters, so the surrounding issue body round-trips exactly.
func MergeManagedSection(body, section, startMarker, endMarker string) string {
	s := strings.Index(body, startMarker)
	e := strings.Index(body, endMarker)
	if s >= 0 && e > s {
		return body[:s] + section + body[e+len(endMarker):]
	}
	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return section + "\n"
	}
	return trimmed + "\n\n" + section + "\n"
}

// RenderSection renders the managed block for the requested section kind from
// the update request payload and returns it with its delimiters.
func RenderSection(req port.IssueProviderUpdateIssueBodySectionRequest, limit int) (section, startMarker, endMarker string, err error) {
	startMarker, endMarker, err = SectionMarkers(req.Section)
	if err != nil {
		return "", "", "", err
	}
	switch req.Section {
	case SectionDevilsAdvocate:
		return RenderDevilsAdvocateSection(req), startMarker, endMarker, nil
	case SectionCompletion:
		if req.Completion == nil {
			return "", "", "", fmt.Errorf("completion payload is required for the completion section")
		}
		section = RenderCompletionSection(*req.Completion)
		// 원고를 잘라 밀어넣지 않는다. 한도를 넘으면 원고를 줄이라고 실패한다(C3-F1).
		if limit > 0 && len(section) > limit {
			return "", "", "", fmt.Errorf("completion section exceeds the body budget (%d > %d); shorten the progress report", len(section), limit)
		}
		return section, startMarker, endMarker, nil
	}
	// SectionMarkers가 이미 거른 kind만 도달하지만, 집합이 닫혀 있음을
	// switch 자체도 강제한다(C3-F5).
	return "", "", "", fmt.Errorf("unsupported issue body section %q", req.Section)
}
