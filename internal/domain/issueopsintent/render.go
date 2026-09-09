// Package issueopsintent renders the requester intent contract as the sealed
// intent artifact. It is a pure projection of record.intent: no clock, no
// filesystem, no JSON, no contract import.
package issueopsintent

import "strings"

// Document is the input of Render: the lifecycle identity plus the recorded
// intent contract fields. The adapter maps record.intent into it.
type Document struct {
	LifecycleID       string
	IssueURL          string
	IntentClass       string
	RawRequest        string
	InterpretedIntent string
	SuccessCriteria   []string
	NonGoals          []string
	Constraints       []string
	Ambiguities       []string
}

// Render returns the intent artifact markdown. Same input, same bytes: the
// contract's recorded_at is deliberately left out because `intent record`
// stamps it anew on every run and the sealed artifact writer is immutable.
func Render(doc Document) string {
	var b strings.Builder
	issue := strings.TrimSpace(doc.IssueURL)
	if issue == "" {
		issue = "(미링크)"
	}
	class := strings.TrimSpace(doc.IntentClass)
	if class == "" {
		class = "standard"
	}
	b.WriteString("# 요청자 의도 계약\n\n")
	b.WriteString("- lifecycle: " + strings.TrimSpace(doc.LifecycleID) + "\n")
	b.WriteString("- issue: " + issue + "\n")
	b.WriteString("- intent_class: " + class + "\n")
	writeSection(&b, "원문 요청", strings.TrimSpace(doc.RawRequest))
	writeSection(&b, "해석", strings.TrimSpace(doc.InterpretedIntent))
	writeList(&b, "성공 기준", doc.SuccessCriteria)
	writeList(&b, "비목표", doc.NonGoals)
	writeList(&b, "제약", doc.Constraints)
	writeList(&b, "모호함", doc.Ambiguities)
	b.WriteString("\n## 읽는 규칙\n")
	b.WriteString("이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.\n")
	return b.String()
}

func writeSection(b *strings.Builder, title, body string) {
	b.WriteString("\n## " + title + "\n")
	b.WriteString(body + "\n")
}

func writeList(b *strings.Builder, title string, items []string) {
	b.WriteString("\n## " + title + "\n")
	written := false
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- " + item + "\n")
		written = true
	}
	if !written {
		b.WriteString("- (없음)\n")
	}
}
