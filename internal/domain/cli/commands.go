package cli

import (
	"strings"

	contract "issueops/internal/contract/cli"
)

// IssueOpsUsageKey는 usage 줄에서 `issueops ` 뒤의 명령 경로를 뽑는다.
// 선택적 플래그는 `[--repo PATH]`, 배타 그룹은 `(--preview|...)`로 표기되므로 그
// 문자로 시작하는 필드도 경로의 끝이다 — 끊지 않으면 `list [--repo`가 경로가 된다.
func IssueOpsUsageKey(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "issueops" {
		return ""
	}
	if !IsLifecycleCommand(fields[1]) {
		return ""
	}
	fields = fields[1:]
	limit := len(fields)
	if limit > 2 {
		limit = 2
	}
	for index, field := range fields[:limit] {
		if strings.HasPrefix(field, "-") || strings.HasPrefix(field, "[") || strings.HasPrefix(field, "(") {
			limit = index
			break
		}
	}
	return strings.Join(fields[:limit], " ")
}

func IsLifecycleCommand(name string) bool {
	for _, command := range contract.LifecycleCommands() {
		if command.Name == name {
			return true
		}
	}
	return false
}
