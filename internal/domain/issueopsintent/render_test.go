package issueopsintent

import (
	"strings"
	"testing"
)

func sampleDocument() Document {
	return Document{
		LifecycleID:       "io-0123456789ab",
		IssueURL:          "https://github.com/acme/repo/issues/42",
		IntentClass:       "standard",
		RawRequest:        "  로그인 실패 시 재시도 안내를 붙여줘  ",
		InterpretedIntent: "만료 토큰 응답에 재발급 안내 필드를 추가한다",
		SuccessCriteria:   []string{"만료 토큰 요청은 401과 안내를 반환한다", "정상 토큰 경로는 그대로다"},
		Constraints:       []string{"DTO 스키마를 바꾸지 않는다"},
		Ambiguities:       []string{"deferred: 안내 문구의 다국어 처리"},
		NonGoals:          []string{"세션 저장소 교체"},
	}
}

func TestRenderIsDeterministic(t *testing.T) {
	doc := sampleDocument()
	first := Render(doc)
	second := Render(doc)
	if first != second {
		t.Fatalf("Render is not deterministic:\n%s\n---\n%s", first, second)
	}
	if !strings.HasSuffix(first, "\n") || strings.HasSuffix(first, "\n\n") {
		t.Fatalf("Render must end with exactly one newline: %q", first[len(first)-3:])
	}
}

func TestRenderCarriesEveryContractFieldAndNoRecordedAt(t *testing.T) {
	out := Render(sampleDocument())
	for _, want := range []string{
		"# 요청자 의도 계약",
		"- lifecycle: io-0123456789ab",
		"- issue: https://github.com/acme/repo/issues/42",
		"- intent_class: standard",
		"## 원문 요청\n로그인 실패 시 재시도 안내를 붙여줘\n",
		"## 해석\n만료 토큰 응답에 재발급 안내 필드를 추가한다\n",
		"## 성공 기준\n- 만료 토큰 요청은 401과 안내를 반환한다\n- 정상 토큰 경로는 그대로다\n",
		"## 비목표\n- 세션 저장소 교체\n",
		"## 제약\n- DTO 스키마를 바꾸지 않는다\n",
		"## 모호함\n- deferred: 안내 문구의 다국어 처리\n",
		"## 읽는 규칙\n",
		"요청자 의도 계약",
		"구현 계약",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered intent is missing %q:\n%s", want, out)
		}
	}
	for _, forbidden := range []string{"recorded_at"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("rendered intent must not carry %q:\n%s", forbidden, out)
		}
	}
}

func TestRenderMarksEmptyListsAndUnlinkedIssue(t *testing.T) {
	doc := sampleDocument()
	doc.IssueURL = ""
	doc.SuccessCriteria = nil
	doc.Constraints = nil
	doc.Ambiguities = nil
	doc.NonGoals = nil
	doc.IntentClass = ""
	out := Render(doc)
	for _, want := range []string{
		"- issue: (미링크)",
		"- intent_class: standard",
		"## 성공 기준\n- (없음)\n",
		"## 비목표\n- (없음)\n",
		"## 제약\n- (없음)\n",
		"## 모호함\n- (없음)\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("rendered intent is missing %q:\n%s", want, out)
		}
	}
}
