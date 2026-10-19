package issueops

import "strings"

// RequiredPlanSections는 issueops-plan 스킬이 계획에 요구하는 네 절이다. 순서는
// 스킬 표와 같고, link-plan이 이 목록으로 계획 본문을 검사한다.
var RequiredPlanSections = []string{
	"## 적용되는 결정과 주의사항",
	"## 재사용하는 기존 구현",
	"## 성능 영향",
	"## 하위 호환성과 side effect",
}

// MissingPlanSections는 계획 본문에 없는 필수 절 제목을 RequiredPlanSections
// 순서로 돌려준다. 제목은 줄 단위로 대조하며 앞뒤 공백만 무시한다. 깊이가 다른
// 헤딩이나 본문 안의 언급은 절로 세지 않는다.
func MissingPlanSections(content string) []string {
	present := map[string]bool{}
	for line := range strings.SplitSeq(content, "\n") {
		present[collapseHeading(line)] = true
	}
	var missing []string
	for _, section := range RequiredPlanSections {
		if !present[collapseHeading(section)] {
			missing = append(missing, section)
		}
	}
	return missing
}

func collapseHeading(line string) string {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "## ") {
		return line
	}
	return "## " + strings.TrimSpace(strings.TrimPrefix(line, "## "))
}

// RequiredArchitectureSections는 issueops-plan 스킬이 아키텍처 문서에 요구하는
// 다섯 절이다. 순서는 스킬의 작성 순서와 같고, architecture review 기록이 이
// 목록으로 문서 본문을 검사한다.
var RequiredArchitectureSections = []string{
	"## 요구사항과 규모",
	"## 데이터와 API 계약",
	"## 구조와 흐름",
	"## 병목과 장애",
	"## 절충점과 기각한 대안",
}

// MissingArchitectureSections는 아키텍처 문서에서 빠졌거나 본문이 비어 있는
// 필수 절 제목을 RequiredArchitectureSections 순서로 돌려준다. 제목 대조는
// MissingPlanSections와 같고, 본문은 다음 `## ` 제목 전까지(하위 제목 포함)의
// 공백이 아닌 글자다. 해당 없는 절은 "해당 없음: 근거"처럼 한 줄을 적는다.
func MissingArchitectureSections(content string) []string {
	hasBody := map[string]bool{}
	current := ""
	for line := range strings.SplitSeq(content, "\n") {
		heading := collapseHeading(line)
		if strings.HasPrefix(heading, "## ") {
			current = heading
			if _, seen := hasBody[current]; !seen {
				hasBody[current] = false
			}
			continue
		}
		if current != "" && strings.TrimSpace(line) != "" {
			hasBody[current] = true
		}
	}
	var missing []string
	for _, section := range RequiredArchitectureSections {
		if !hasBody[collapseHeading(section)] {
			missing = append(missing, section)
		}
	}
	return missing
}
