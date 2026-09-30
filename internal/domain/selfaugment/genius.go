package selfaugment

import "strings"

func SelectGeniusFormulas(text string) []string {
	if strings.TrimSpace(text) == "" {
		return []string{}
	}
	formulas := []string{
		"문제 재정의 알고리즘",
		"혁신적 솔루션 생성 공식",
		"사고의 진화 방정식",
		"복잡성 해결 매트릭스",
	}
	selected := []string{}
	for _, formula := range formulas {
		if strings.Contains(text, formula) {
			selected = append(selected, formula)
		}
	}
	return selected
}
