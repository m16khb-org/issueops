package preflight

import (
	"fmt"
	preflightcontract "issueops/internal/contract/preflight"
	"regexp"
	"strings"
)

func listRemotes(remoteV string) []preflightcontract.RemoteInfo {
	lines := splitLines(remoteV)
	var out []preflightcontract.RemoteInfo
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "(fetch)" {
			out = append(out, preflightcontract.RemoteInfo{Name: fields[0], URL: redactRemote(fields[1])})
		}
	}
	return out
}

var historyArgs = []string{"log", "-10", "--format=%h%x00%s%x00%B%x00"}

type historyRecord struct{ sha, subject, body string }

type commitHistory []historyRecord

// parseHistory reads NUL-terminated sha/subject/body triples; Git separates
// records with one newline. An incomplete trailing triple is dropped.
func parseHistory(raw string) commitHistory {
	fields := strings.Split(raw, "\x00")
	var history commitHistory
	for i := 0; i+2 < len(fields); i += 3 {
		history = append(history, historyRecord{
			sha:     strings.TrimPrefix(fields[i], "\n"),
			subject: fields[i+1],
			body:    fields[i+2],
		})
	}
	return history
}

func (history commitHistory) last() string {
	if len(history) == 0 {
		return ""
	}
	return strings.TrimSpace(history[0].sha + " " + history[0].subject)
}

func (history commitHistory) commits(limit int) []preflightcontract.CommitInfo {
	var out []preflightcontract.CommitInfo
	for _, record := range history[:min(limit, len(history))] {
		out = append(out, preflightcontract.CommitInfo{SHA: record.sha, Subject: record.subject})
	}
	return out
}

func (history commitHistory) bodies() []string {
	if len(history) == 0 {
		return []string{""}
	}
	out := make([]string, 0, len(history))
	for _, record := range history {
		out = append(out, record.body)
	}
	return out
}

func redactRemote(url string) string {
	httpUserInfo := regexp.MustCompile(`(https?://)[^/@]+@`)
	url = httpUserInfo.ReplaceAllString(url, `${1}<redacted>@`)
	creds := regexp.MustCompile(`(://)([^:/@]+):([^/@]+)@`)
	return creds.ReplaceAllString(url, `${1}<redacted>:<redacted>@`)
}

func splitLines(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	return strings.Split(strings.TrimRight(s, "\n"), "\n")
}

func atoi(s string) int {
	var n int
	_, _ = fmt.Sscanf(s, "%d", &n)
	return n
}
