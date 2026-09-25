package issueopspreparation

const (
	// IssueOps implementer(하위 세션 execution owner)의 host별 기본 모델.
	// execution prepare가 --owner-model/--owner-effort 미지정 호출에 적용한다.
	ImplementerModelCodex  = "gpt-6-sol"
	ImplementerEffortCodex = "high"
	// Claude Code 자동 체인은 Opus 5 planner가 계획·리뷰하고 Sonnet 5
	// implementer가 실행한다. Fable 5는 명시적 수동 지정에만 사용한다.
	ImplementerModelClaude = "claude-sonnet-5"
	// claude CLI의 --effort <level> 플래그 실지원을 확인함(2026-07-24).
	// 플래그가 제거되면 ownerAgentCommand(adapter/orca/client.go)의 claude
	// 분기에서 effort 인자를 조건부 생략으로 되돌린다.
	ImplementerEffortClaude = "high"
	ImplementerModelOmo     = "openai-codex/gpt-5.6-sol"
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
