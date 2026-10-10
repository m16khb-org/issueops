package projectdoc

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
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
// Summary line fall back to their frontmatter description.
func ParseRecord(rel, content string) Record {
	record := Record{Rel: rel}
	description := ""
	for line := range strings.Lines(content) {
		line = strings.TrimSpace(line)
		switch {
		case record.Title == "" && strings.HasPrefix(line, "# "):
			record.Title = stripLeadingDate(strings.TrimSpace(line[2:]))
		case record.Summary == "" && strings.HasPrefix(line, "- Summary:"):
			record.Summary = strings.TrimSpace(strings.TrimPrefix(line, "- Summary:"))
		case record.Source == "" && strings.HasPrefix(line, "- Source:"):
			record.Source = strings.TrimSpace(strings.TrimPrefix(line, "- Source:"))
		case description == "" && strings.HasPrefix(line, "description:"):
			description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		}
	}
	if record.Summary == "" {
		record.Summary = description
	}
	return record
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
	type match struct {
		record Record
		score  float64
		shared []string
	}
	var matches []match
	for i, record := range records {
		var shared []string
		score := 0.0
		for term := range taskTerms {
			if recordTerms[i][term] {
				shared = append(shared, term)
				score += math.Log(float64(len(records)) / float64(df[term]))
			}
		}
		if len(shared) >= 2 {
			sort.Strings(shared)
			matches = append(matches, match{record, score, shared})
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
		docs = append(docs, RouteDoc{m.record.Rel, fmt.Sprintf("past record %q shares: %s", m.record.Title, strings.Join(m.shared, ", "))})
	}
	return docs
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
			terms[english] = true
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

func latinTerm(word string) (string, bool) {
	if len(word) < 3 || englishStopWords[word] {
		return "", false
	}
	for _, suffix := range []struct {
		text   string
		minLen int
	}{{"ment", 7}, {"ing", 6}, {"ed", 5}, {"s", 4}} {
		if len(word) > suffix.minLen && strings.HasSuffix(word, suffix.text) && !(suffix.text == "s" && strings.HasSuffix(word, "ss")) {
			return word[:len(word)-len(suffix.text)], true
		}
	}
	return word, true
}
