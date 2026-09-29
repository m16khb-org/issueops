package preflight

import (
	"fmt"
	preflightcontract "issueops/internal/contract/preflight"
	"regexp"
	"strings"
)

func listRemotes(root string) []preflightcontract.RemoteInfo {
	lines := splitLines(GitOut(root, "remote", "-v"))
	var out []preflightcontract.RemoteInfo
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 && fields[2] == "(fetch)" {
			out = append(out, preflightcontract.RemoteInfo{Name: fields[0], URL: redactRemote(fields[1])})
		}
	}
	return out
}

func recentCommits(root string, limit int) []preflightcontract.CommitInfo {
	lines := splitLines(GitOut(root, "log", fmt.Sprintf("-%d", limit), "--pretty=format:%h%x09%s"))
	var out []preflightcontract.CommitInfo
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 {
			out = append(out, preflightcontract.CommitInfo{SHA: parts[0], Subject: parts[1]})
		}
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
