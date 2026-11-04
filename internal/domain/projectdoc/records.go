package projectdoc

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

// Record is the routing view of one dated record file under a family module
// directory: what a task description is compared against.
type Record struct{ Rel, Title, Summary, Source string }

// RecordModuleDirs are the family module directories whose dated records
// route-docs searches. Records there are append-only history that the family
// indexes do not list one by one.
func RecordModuleDirs() []string { return []string{"cautions", "adr"} }

// IsRecordFile reports whether name is a dated record file such as
// "2026-09-07-slug.md", as opposed to an overview or a living module.
func IsRecordFile(name string) bool {
	if len(name) < len("2006-01-02-x.md") || name[10] != '-' || !strings.HasSuffix(name, ".md") {
		return false
	}
	_, err := time.Parse("2006-01-02", name[:10])
	return err == nil
}

// ParseRecord reads the first H1 (without a leading "YYYY-MM-DD —" date), the
// "- Summary:" line, and the "- Source:" line. Hand-written records without a
// Summary line fall back to their frontmatter description, and records with
// neither fall back to the start of their decision ("## Decision"/"## 결정"
// section or a "**결정:**" line), so older records are matched on what they
// decided and not on the title alone.
func ParseRecord(rel, content string) Record {
	p := &recordParser{record: Record{Rel: rel}}
	for line := range strings.Lines(content) {
		p.feed(strings.TrimSpace(line))
	}
	return p.finish()
}

// recordParser holds the line-by-line state ParseRecord accumulates.
type recordParser struct {
	record      Record
	description string
	decision    strings.Builder
	inDecision  bool
	// inDecisionLine keeps a one-line decision marker open across its wrapped lines, up to
	// a blank line or the next list item, bold label, or heading.
	inDecisionLine bool
}

// feed consumes one trimmed line. The first matching rule wins, in this order:
// a wrapped decision line, a record field, a "## " heading, a one-line decision
// marker, and a line inside a decision section.
func (p *recordParser) feed(line string) {
	continued := p.continuesDecisionLine(line)
	p.inDecisionLine = false
	switch {
	case continued:
		p.inDecisionLine = true
		if p.decisionRoom() {
			p.decision.WriteString(" " + line)
		}
	case p.takeField(line):
	case strings.HasPrefix(line, "## "):
		p.inDecision = p.opensDecisionSection(strings.ToLower(strings.TrimSpace(line[3:])))
	case p.takeDecisionLine(line):
	case p.inDecision && line != "" && p.decisionRoom():
		p.decision.WriteString(line + " ")
	}
}

// continuesDecisionLine reports whether line wraps the open one-line decision marker.
func (p *recordParser) continuesDecisionLine(line string) bool {
	return p.inDecisionLine && line != "" && !strings.HasPrefix(line, "- ") &&
		!strings.HasPrefix(line, "**") && !strings.HasPrefix(line, "#")
}

// decisionRoom reports whether the decision text is still below the fallback bound.
func (p *recordParser) decisionRoom() bool {
	return utf8.RuneCountInString(p.decision.String()) < decisionFallbackRunes
}

// opensDecisionSection reports whether a lowercased "## " heading starts the
// decision section, which only counts while no decision has been collected.
func (p *recordParser) opensDecisionSection(heading string) bool {
	return p.decision.Len() == 0 && (strings.HasPrefix(heading, "decision") || strings.HasPrefix(heading, "결정") || heading == "summary")
}

// takeField records the first H1, "- Summary:", "- Source:", and
// "description:" lines and reports whether line was one of them.
func (p *recordParser) takeField(line string) bool {
	switch {
	case p.record.Title == "" && strings.HasPrefix(line, "# "):
		p.record.Title = stripLeadingDate(strings.TrimSpace(line[2:]))
	case p.record.Summary == "" && strings.HasPrefix(line, "- Summary:"):
		p.record.Summary = strings.TrimSpace(strings.TrimPrefix(line, "- Summary:"))
	case p.record.Source == "" && strings.HasPrefix(line, "- Source:"):
		p.record.Source = strings.TrimSpace(strings.TrimPrefix(line, "- Source:"))
	case p.description == "" && strings.HasPrefix(line, "description:"):
		p.description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
	default:
		return false
	}
	return true
}

// takeDecisionLine starts the decision text from a one-line decision marker
// when no decision has been collected yet, and reports whether it did.
func (p *recordParser) takeDecisionLine(line string) bool {
	if p.decision.Len() != 0 {
		return false
	}
	match := decisionLineRe().FindStringSubmatch(line)
	if match == nil {
		return false
	}
	p.decision.WriteString(match[1])
	p.inDecisionLine = true
	return true
}

// finish settles the summary: the Summary line, else the frontmatter
// description, else the start of the decision.
func (p *recordParser) finish() Record {
	switch {
	case p.record.Summary != "":
	case p.description != "":
		p.record.Summary = p.description
	default:
		p.record.Summary = truncateRunes(strings.TrimSpace(p.decision.String()), decisionFallbackRunes)
	}
	return p.record
}

// decisionLineRe is a one-line decision marker: "**결정:** text", "**결정: text**",
// "Decision: text", "- Decision: text", or "**Decision:** text". It captures the text.
var decisionLineRe = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^(?:-\s+)?(?:\*\*)?(?i:decision|결정)(?:\*\*)?\s*:\s*(?:\*\*)?\s*(.*?)\s*(?:\*\*)?$`)
})

// decisionFallbackRunes bounds the decision text a record without a summary is
// matched on: about one summary sentence, so long decisions do not match every task.
const decisionFallbackRunes = 300

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit])
}

func stripLeadingDate(title string) string {
	if len(title) < 10 {
		return title
	}
	if _, err := time.Parse("2006-01-02", title[:10]); err != nil {
		return title
	}
	return strings.TrimLeft(title[10:], " —–-:")
}

// RecordIndexLine is the one-line link append adds to a family module overview.
func RecordIndexLine(title, fileName string) string {
	title = strings.NewReplacer("[", `\[`, "]", `\]`).Replace(strings.TrimSpace(title))
	return fmt.Sprintf("- [%s](%s)", title, fileName)
}

// MatchRecords returns up to limit records that share at least two distinct
// terms with the task, strongest first and newest first on ties. A shared term
// weighs ln(N/df) over the given records, so words most records use barely
// count. Korean task words reach English records through koreanTerms.
func MatchRecords(task string, records []Record, limit int) []RouteDoc {
	return matchRecordsWith(task, records, limit, "past record %q shares: %s", false)
}

// MatchModules is MatchRecords for the living modules of a routed index doc:
// the module files its table points to, matched by title, description, and
// section headings, so a task reaches the module that owns the detailed rule.
func MatchModules(task string, modules []Record, limit int) []RouteDoc {
	return matchRecordsWith(task, modules, limit, "module %q of the routed index shares: %s", true)
}

// ParseModule is the routing view of a living module: its H1, frontmatter
// description, and "## " headings, which name the rules the module owns.
func ParseModule(rel, content string) Record {
	record := Record{Rel: rel}
	var summary []string
	for line := range strings.Lines(content) {
		line = strings.TrimSpace(line)
		switch {
		case record.Title == "" && strings.HasPrefix(line, "# "):
			record.Title = strings.TrimSpace(line[2:])
		case strings.HasPrefix(line, "## "):
			summary = append(summary, strings.TrimSpace(line[3:]))
		case strings.HasPrefix(line, "description:"):
			summary = append(summary, strings.TrimSpace(strings.TrimPrefix(line, "description:")))
		}
	}
	record.Summary = strings.Join(summary, "; ")
	return record
}

// recordMatch is a record that shares at least two terms with the task.
type recordMatch struct {
	record Record
	score  float64
	shared []string
}

func matchRecordsWith(task string, records []Record, limit int, reasonFormat string, skipCommon bool) []RouteDoc {
	taskTerms := routeTerms(task)
	if len(taskTerms) < 2 || len(records) == 0 || limit <= 0 {
		return nil
	}
	recordTerms := make([]map[string]bool, len(records))
	df := map[string]int{}
	for i, record := range records {
		recordTerms[i] = routeTerms(record.Title + " " + record.Summary + " " + record.Source)
		for term := range recordTerms[i] {
			df[term]++
		}
	}
	var matches []recordMatch
	for i, record := range records {
		shared, score := sharedRecordTerms(taskTerms, recordTerms[i], df, len(records), skipCommon)
		if len(shared) >= 2 {
			sort.Strings(shared)
			matches = append(matches, recordMatch{record, score, shared})
		}
	}
	sort.Slice(matches, func(a, b int) bool {
		if matches[a].score != matches[b].score {
			return matches[a].score > matches[b].score
		}
		return matches[a].record.Rel > matches[b].record.Rel
	})
	docs := make([]RouteDoc, 0, min(limit, len(matches)))
	for _, m := range matches[:min(limit, len(matches))] {
		docs = append(docs, RouteDoc{m.record.Rel, fmt.Sprintf(reasonFormat, m.record.Title, strings.Join(m.shared, ", "))})
	}
	return docs
}

// sharedRecordTerms returns the task terms one record carries and their summed
// inverse-document-frequency score.
func sharedRecordTerms(taskTerms, recordTerms map[string]bool, df map[string]int, total int, skipCommon bool) ([]string, float64) {
	var shared []string
	score := 0.0
	for term := range taskTerms {
		// For modules, a term a third of the candidates carry (the project's own
		// name, for one) cannot tell them apart, so it does not count toward the two.
		if recordTerms[term] && (!skipCommon || df[term]*3 <= total) {
			shared = append(shared, term)
			score += math.Log(float64(total) / float64(df[term]))
		}
	}
	return shared, score
}

// koreanTerms maps Korean words to the English term records use, so a Korean
// task reaches English records. A Korean word containing a key yields only the
// mapped term.
var koreanTerms = map[string]string{
	"배포": "deploy", "빌드": "build", "이미지": "image", "컨테이너": "container", "디스크": "disk",
	"메모리": "memory", "서버": "server", "마이그레이션": "migration", "데이터베이스": "database",
	"테스트": "test", "커밋": "commit", "브랜치": "branch", "머지": "merge", "병합": "merge",
	"훅": "hook", "스킬": "skill", "캐시": "cache", "로그": "log", "설치": "install",
	"업데이트": "update", "권한": "permission", "인증": "auth", "세션": "session", "워크트리": "worktree",
	"의존성": "dependency", "성능": "performance", "보안": "security", "백업": "backup", "롤백": "rollback",
	"프롬프트": "prompt", "모델": "model", "스키마": "schema", "문서": "doc", "검증": "verify",
	"리뷰": "review", "이슈": "issue", "잠금": "lock",
}

// Korean word stems too common to tell records apart.
var koreanStopStems = map[string]bool{
	"한다": true, "하는": true, "하고": true, "하지": true, "했다": true, "하면": true, "해야": true,
	"있다": true, "있는": true, "없다": true, "없는": true, "된다": true, "되는": true, "되어": true,
	"이다": true, "이번": true, "모든": true, "그리": true, "위해": true, "대한": true, "통해": true,
	"경우": true, "때문": true, "같은": true, "다른": true, "이미": true, "아직": true, "다시": true,
	"먼저": true, "직접": true,
	// Task-description filler: every change "adds", "changes", or "summarizes" something, and every
	// stage task is a "plan" for some "problem".
	"추가": true, "변경": true, "요약": true, "결과": true, "에서": true, "경로": true, "명령": true,
	"상태": true, "수정": true, "구현": true, "완료": true, "사이": true, "정리": true, "계획": true, "문제": true,
}

var englishStopWords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "from": true, "into": true, "that": true,
	"this": true, "before": true, "after": true, "was": true, "were": true, "are": true, "not": true,
	"but": true, "all": true, "any": true, "its": true, "has": true, "have": true, "had": true,
	"when": true, "then": true, "than": true, "also": true, "only": true, "out": true, "via": true,
	"per": true, "one": true, "two": true, "use": true, "used": true, "uses": true, "using": true,
	"can": true, "must": true, "should": true, "will": true,
}

// routeTerms splits text into Latin/digit words and Hangul words. Latin words
// shorter than three letters or in englishStopWords are dropped and lightly
// stemmed; Hangul words map through koreanTerms or else keep their first two
// syllables, which drops attached particles and endings ("서버에" → "서버").
func routeTerms(text string) map[string]bool {
	terms := map[string]bool{}
	var word []rune
	hangulWord := false
	flush := func() {
		if len(word) == 0 {
			return
		}
		if hangulWord {
			addHangulTerms(terms, string(word))
		} else if term, ok := latinTerm(string(word)); ok {
			terms[term] = true
		}
		word = word[:0]
	}
	for _, r := range strings.ToLower(text) {
		isHangul := unicode.Is(unicode.Hangul, r)
		isLatin := !isHangul && (unicode.IsLetter(r) || unicode.IsDigit(r))
		if !isHangul && !isLatin {
			flush()
			continue
		}
		if len(word) > 0 && isHangul != hangulWord {
			flush()
		}
		hangulWord = isHangul
		word = append(word, r)
	}
	flush()
	return terms
}

func addHangulTerms(terms map[string]bool, word string) {
	mapped := false
	for korean, english := range koreanTerms {
		if strings.Contains(word, korean) {
			// The English side goes through the same stemming as Latin text,
			// so a Korean task meets a record that spells the word in English.
			if term, ok := latinTerm(english); ok {
				terms[term] = true
			}
			mapped = true
		}
	}
	runes := []rune(word)
	if mapped || len(runes) < 2 {
		return
	}
	if stem := string(runes[:2]); !koreanStopStems[stem] {
		terms[stem] = true
	}
}

// latinTerm lightly stems one lowercase Latin word so a plural, its singular,
// and common inflections reach the same term. After the suffix is dropped, a
// final consonant+"y" and a final "ie" both become "i" ("retry", "retries",
// "retried", "retrying" -> "retri"; "zombie", "zombies" -> "zombi"), and a final
// consonant+"che" drops its "e" to meet the "-ches" plural ("avalanche",
// "avalanches" -> "avalanch"). The -es rules drop "es" only after ss, x, sh, or
// a consonant+ch; "caches" and "sizes" fall through to the plain -s rule, so
// they still meet "cache" and "size".
func latinTerm(word string) (string, bool) {
	if len(word) < 3 || englishStopWords[word] {
		return "", false
	}
	return normalizeLatinEnding(dropLatinSuffix(word)), true
}

func dropLatinSuffix(word string) string {
	if len(word) > 4 {
		switch {
		case strings.HasSuffix(word, "ies"):
			return word[:len(word)-3] + "i"
		case strings.HasSuffix(word, "sses"), strings.HasSuffix(word, "xes"), strings.HasSuffix(word, "shes"), consonantBefore(word, "ches"):
			return word[:len(word)-2]
		}
	}
	for _, suffix := range []struct {
		text   string
		minLen int
	}{{"ment", 7}, {"ing", 6}, {"ed", 5}, {"s", 4}} {
		if len(word) > suffix.minLen && strings.HasSuffix(word, suffix.text) && !(suffix.text == "s" && strings.HasSuffix(word, "ss")) {
			return word[:len(word)-len(suffix.text)]
		}
	}
	return word
}

func normalizeLatinEnding(stem string) string {
	switch {
	case len(stem) >= 4 && consonantBefore(stem, "y"):
		return stem[:len(stem)-1] + "i"
	case len(stem) > 4 && strings.HasSuffix(stem, "ie"):
		return stem[:len(stem)-1]
	case len(stem) > 5 && consonantBefore(stem, "che"):
		return stem[:len(stem)-1]
	}
	return stem
}

// consonantBefore reports whether word ends in suffix preceded by a consonant:
// "batches" and "branches" for "ches", "retry" for "y". After a vowel the base
// differs ("caches" is "cache", "key" stays "key").
func consonantBefore(word, suffix string) bool {
	if !strings.HasSuffix(word, suffix) || len(word) <= len(suffix) {
		return false
	}
	return !strings.ContainsRune("aeiou", rune(word[len(word)-len(suffix)-1]))
}
