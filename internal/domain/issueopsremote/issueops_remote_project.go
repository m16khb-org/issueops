package remote

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

var scpRemotePattern = sync.OnceValue(func() *regexp.Regexp { return regexp.MustCompile(`^[^@/:]+@([^/:[\]]+):(.+)$`) })

func ValidateChildMatchesParent(parentURL, childURL string) error {
	provider := ProviderFromURL(parentURL)
	if provider == "" {
		return nil
	}
	if IssueNumber(childURL) == "" {
		return fmt.Errorf("child issue url must be a numeric %s issue or work item URL", provider)
	}
	parentKey := ProjectKey(parentURL, provider, "issue")
	childKey := ProjectKey(childURL, provider, "issue")
	if parentKey == "" || childKey == "" {
		return fmt.Errorf("child issue url must be a %s issue URL", provider)
	}
	if parentKey != childKey {
		return fmt.Errorf("child issue url must match linked parent issue project")
	}
	return nil
}

func ProjectKey(rawURL, provider, kind string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || !isCanonicalWebURL(parsed) {
		return ""
	}
	host := normalizedWebHost(parsed)
	parts := SplitURLPath(strings.ToLower(parsed.EscapedPath()))
	switch provider + ":" + kind {
	case "github:issue":
		if !isGitHubItemPath(parts, "issues") {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "github:pr":
		if !isGitHubItemPath(parts, "pull") {
			return ""
		}
		return host + "/" + strings.Join(parts[:2], "/")
	case "gitlab:issue":
		if !isGitLabItemPath(parts, parsed.Hostname(), isGitLabIssueLikePath) {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	case "gitlab:mr":
		if !isGitLabItemPath(parts, parsed.Hostname(), isGitLabMergeRequestPath) {
			return ""
		}
		return host + "/" + strings.Join(parts[:len(parts)-3], "/")
	default:
		return ""
	}
}

// isCanonicalWebURL는 query/fragment/userinfo 없는 https URL인지 확인한다.
func isCanonicalWebURL(parsed *url.URL) bool {
	return parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && parsed.Scheme == "https"
}

// isGitHubItemPath는 owner/repo/<segment>/<number> 형태인지 확인한다.
func isGitHubItemPath(parts []string, segment string) bool {
	return len(parts) == 4 && parts[2] == segment && isDecimalString(parts[3])
}

// isGitLabItemPath는 <group...>/-/<kind>/<number> 형태이고 호스트가 github.com이 아닌지 확인한다.
func isGitLabItemPath(parts []string, hostname string, kindOK func(string) bool) bool {
	if strings.EqualFold(hostname, "github.com") || len(parts) < 5 {
		return false
	}
	return parts[len(parts)-3] == "-" && kindOK(parts[len(parts)-2]) && isDecimalString(parts[len(parts)-1])
}

func isGitLabMergeRequestPath(segment string) bool { return segment == "merge_requests" }

func ProjectKeyFromGitRemoteURL(rawURL, provider string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" || strings.ContainsAny(rawURL, "\x00\r\n\t ") {
		return "", fmt.Errorf("git remote URL is empty or contains whitespace")
	}
	host, remotePath, err := gitRemoteHostAndPath(rawURL)
	if err != nil {
		return "", err
	}
	remotePath = strings.TrimSuffix(strings.Trim(remotePath, "/"), ".git")
	parts := SplitURLPath(strings.ToLower(remotePath))
	if provider == "github" {
		if len(parts) != 2 {
			return "", fmt.Errorf("git remote URL does not identify one GitHub project")
		}
	} else if provider == "gitlab" {
		if host == "github.com" || len(parts) < 2 {
			return "", fmt.Errorf("git remote URL does not identify one GitLab project")
		}
	} else {
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
	return host + "/" + strings.Join(parts, "/"), nil
}

// gitRemoteHostAndPath는 HTTPS/SSH URL 또는 SCP 문법의 git remote에서 호스트와 경로를 분리한다.
func gitRemoteHostAndPath(rawURL string) (host, remotePath string, err error) {
	if parsed, perr := url.Parse(rawURL); perr == nil && parsed.Scheme != "" {
		if parsed.Scheme != "https" && parsed.Scheme != "ssh" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
			return "", "", fmt.Errorf("git remote URL scheme or authority is unsupported")
		}
		if parsed.Scheme == "https" {
			if parsed.User != nil {
				return "", "", fmt.Errorf("git remote HTTPS URL must not contain userinfo")
			}
			host = normalizedWebHost(parsed)
		} else {
			if _, hasPassword := parsed.User.Password(); hasPassword {
				return "", "", fmt.Errorf("git remote SSH URL must not contain password userinfo")
			}
			host = strings.ToLower(parsed.Hostname())
		}
		return host, strings.TrimPrefix(parsed.EscapedPath(), "/"), nil
	}
	if match := scpRemotePattern().FindStringSubmatch(rawURL); len(match) == 3 {
		return strings.ToLower(match[1]), match[2], nil
	}
	return "", "", fmt.Errorf("git remote URL must be HTTPS, SSH, or SCP syntax")
}

func normalizedWebHost(parsed *url.URL) string {
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	// net.JoinHostPort와 같은 규칙이지만 net을 import하지 않는다. 호스트 정규화는
	// 순수 문자열 규칙이고, domain은 net을 의존해서는 안 된다. 아래 IPv6 분기가
	// 이미 같은 대괄호 처리를 하므로 표기도 일관된다.
	if port != "" && port != "443" {
		if strings.Contains(host, ":") {
			return "[" + host + "]:" + port
		}
		return host + ":" + port
	}
	if strings.Contains(host, ":") {
		return "[" + host + "]"
	}
	return host
}

func ProviderFromURL(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	host := strings.ToLower(parsed.Hostname())
	path := strings.ToLower(parsed.Path)
	if host == "github.com" && strings.Contains(path, "/issues/") {
		return "github"
	}
	if strings.Contains(host, "gitlab") || strings.Contains(path, "/-/issues/") || strings.Contains(path, "/-/work_items/") {
		return "gitlab"
	}
	return ""
}
func IssueNumber(issueURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(issueURL))
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	for i, part := range parts {
		if isGitLabIssueLikePath(part) && i+1 < len(parts) {
			number := parts[i+1]
			for _, r := range number {
				if r < '0' || r > '9' {
					return ""
				}
			}
			return number
		}
	}
	return ""
}

func isGitLabIssueLikePath(part string) bool {
	return part == "issues" || part == "work_items"
}

// TrackedMaterialNames are the tracked copies of the implementation materials
// that phase transitions write next to gates.md in .issueops/issues/<n>/
// (#513). They are derived from the sealed plan and the record.
var TrackedMaterialNames = []string{"plan.md", "intent.md", "spec.md", "architecture.md", "plan-review.md"}

// IsTrackedMaterialPath reports whether relPath (worktree-relative, slash or
// OS separators) is one of issueURL's tracked material copies.
func IsTrackedMaterialPath(issueURL, relPath string) bool {
	n := IssueNumber(issueURL)
	if n == "" {
		return false
	}
	relPath = strings.ReplaceAll(relPath, "\\", "/")
	for _, name := range TrackedMaterialNames {
		if relPath == ".issueops/issues/"+n+"/"+name {
			return true
		}
	}
	return false
}

// IssueArtifactDir은 linked issue URL로 결정하는 봉인 아티팩트 디렉터리
// (워크트리 상대, slash)다: `.issueops/issues/<n>/artifact`. 번호를 알 수
// 없으면 빈 문자열이며, execution prepare는 그 경우를 거부한다(#482).
func IssueArtifactDir(issueURL string) string {
	if n := IssueNumber(issueURL); n != "" {
		return ".issueops/issues/" + n + "/artifact"
	}
	return ""
}
