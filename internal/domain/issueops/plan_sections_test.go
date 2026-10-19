package issueops

import (
	"reflect"
	"strings"
	"testing"
)

// 계획의 네 필수 절은 issueops-plan 스킬이 정한 판단 기록이다. link-plan이
// 그 존재를 검사하므로 제목 대조는 domain이 소유한다.
func TestMissingPlanSectionsReportsAbsentHeadings(t *testing.T) {
	content := strings.Join([]string{
		"# 계획",
		"## 적용되는 결정과 주의사항",
		"- 대조했으나 없음",
		"##  성능 영향  ",
	}, "\n")
	got := MissingPlanSections(content)
	want := []string{"## 재사용하는 기존 구현", "## 하위 호환성과 side effect"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingPlanSections=%v, want %v", got, want)
	}
}

func TestMissingPlanSectionsIsEmptyWhenAllPresent(t *testing.T) {
	var lines []string
	for _, section := range RequiredPlanSections {
		lines = append(lines, section, "본문")
	}
	if got := MissingPlanSections(strings.Join(lines, "\n")); len(got) != 0 {
		t.Fatalf("all sections present, got missing %v", got)
	}
}

func TestMissingPlanSectionsIgnoresDeeperHeadingsAndInlineMentions(t *testing.T) {
	content := "### 적용되는 결정과 주의사항\n본문에 ## 성능 영향 이라는 글자만 있음\n"
	got := MissingPlanSections(content)
	if len(got) != len(RequiredPlanSections) {
		t.Fatalf("deeper heading or inline mention must not count: %v", got)
	}
}

func TestMissingArchitectureSectionsRejectsEmptyAndWhitespaceBodies(t *testing.T) {
	content := strings.Join([]string{
		"# 아키텍처",
		"## 요구사항과 규모",
		"- 호출자 1명, 사이클당 문서 1개",
		"## 데이터와 API 계약",
		"",
		"   ",
		"## 구조와 흐름",
		"해당 없음: 새 구성요소가 없다",
		"## 병목과 장애",
		"## 절충점과 기각한 대안",
		"- 새 durable phase: 기각",
	}, "\n")
	got := MissingArchitectureSections(content)
	want := []string{"## 데이터와 API 계약", "## 병목과 장애"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MissingArchitectureSections=%v, want %v", got, want)
	}
}

func TestMissingArchitectureSectionsIsEmptyWhenAllPresent(t *testing.T) {
	var lines []string
	for _, section := range RequiredArchitectureSections {
		lines = append(lines, section, "본문")
	}
	if got := MissingArchitectureSections(strings.Join(lines, "\n")); len(got) != 0 {
		t.Fatalf("all sections present, got missing %v", got)
	}
}

func TestMissingArchitectureSectionsIgnoresDeeperHeadingsAndInlineMentions(t *testing.T) {
	content := "### 요구사항과 규모\n본문에 ## 병목과 장애 라는 글자만 있음\n"
	got := MissingArchitectureSections(content)
	if len(got) != len(RequiredArchitectureSections) {
		t.Fatalf("deeper heading or inline mention must not count: %v", got)
	}
}

func TestMissingArchitectureSectionsCountsSubheadingContentAsBody(t *testing.T) {
	var lines []string
	for _, section := range RequiredArchitectureSections {
		lines = append(lines, section, "### 세부", "내용")
	}
	if got := MissingArchitectureSections(strings.Join(lines, "\n")); len(got) != 0 {
		t.Fatalf("sub-heading content belongs to the section body, got missing %v", got)
	}
}
