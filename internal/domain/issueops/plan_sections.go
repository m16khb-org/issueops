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
	for _, line := range strings.Split(content, "\n") {
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
