package agentmodel

import "strings"

const (
	// IssueOps implementer(하위 세션 execution owner)의 host별 기본 모델.
	// execution prepare가 --owner-model/--owner-effort 미지정 호출에 적용한다.
	ImplementerModelCodex  = "gpt-6-sol"
	ImplementerEffortCodex = "high"
	// Claude Code 자동 체인은 Opus 5.5 planner가 계획·리뷰하고 Sonnet 5.5
	// implementer가 실행한다.
	ImplementerModelClaude = "claude-sonnet-5-5"
	// claude CLI의 --effort <level> 플래그 실지원을 확인함(2026-07-24).
	// 플래그가 제거되면 ownerAgentCommand(adapter/orca/client.go)의 claude
	// 분기에서 effort 인자를 조건부 생략으로 되돌린다.
	ImplementerEffortClaude = "high"
	ImplementerModelOmo     = "chatgpt-subscription/gpt-6-sol"
	ImplementerEffortOmo    = "max"
)

func ImplementerDefaults(host string) (model string, effort string, ok bool) {
	switch host {
	case "codex":
		return ImplementerModelCodex, ImplementerEffortCodex, true
	case "claude":
		return ImplementerModelClaude, ImplementerEffortClaude, true
	case "omo":
		return ImplementerModelOmo, ImplementerEffortOmo, true
	default:
		return "", "", false
	}
}

// PlannerDefaults selects the model for planning and independent gate reviews.
func PlannerDefaults(host string) (model, effort string, ok bool) {
	switch host {
	case "codex":
		return "gpt-6-astra", "xhigh", true
	case "claude":
		return "claude-opus-5-5", "high", true
	case "omo":
		return "chatgpt-subscription/gpt-6-astra", "max", true
	default:
		return "", "", false
	}
}

const ReviewEffortDocsOnly = "medium"

// ReviewEffortForTier lowers effort only for documentation-only changes.
func ReviewEffortForTier(host, tier string) string {
	_, effort, ok := PlannerDefaults(host)
	if !ok {
		return ""
	}
	if strings.TrimSpace(tier) == "docs-only" {
		return ReviewEffortDocsOnly
	}
	return effort
}

// ResearchDefaults is for bounded, read-only research, never gate reviews.
// Returning a model does not authorize delegating work to another agent.
func ResearchDefaults(host string) (model, effort string, ok bool) {
	switch host {
	case "codex":
		return "gpt-6-luna", "medium", true
	case "omo":
		return "chatgpt-subscription/gpt-6-luna", "medium", true
	default:
		return "", "", false
	}
}
