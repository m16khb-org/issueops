// Package remoteparse parses git remote and provider URLs into structured
// parts, including GitLab merge request and issue paths. It is a pure domain
// package.
package remoteparse

import (
	"net/url"
	"path"
	"strconv"
	"strings"
)

// GitLabMRPathParts is the project path and IID parsed from a GitLab merge
// request URL path. Both are empty when the path is not a merge request.
type GitLabMRPathParts struct {
	Project string
	IID     string
}

// GitLabIssuePathParts is the project path, IID, and kind ("issues" or
// "work_items") parsed from a GitLab issue URL path. All are empty when the
// path is not an issue.
type GitLabIssuePathParts struct {
	Project string
	IID     string
	Kind    string
}

// SplitGitLabMRPath parses an escaped GitLab URL path of the form
// <project...>/-/merge_requests/<iid>. The project, which may be a nested
// group path, is URL-unescaped. A missing marker or non-numeric IID returns
// the zero value.
func SplitGitLabMRPath(escapedPath string) GitLabMRPathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || parts[i+1] != "merge_requests" {
			continue
		}
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return GitLabMRPathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return GitLabMRPathParts{Project: strings.Join(projectParts, "/"), IID: iid}
	}
	return GitLabMRPathParts{}
}

// SplitGitLabIssuePath parses an escaped GitLab URL path of the form
// <project...>/-/issues/<iid> or <project...>/-/work_items/<iid>. The project
// is URL-unescaped and Kind records which marker matched. A missing marker or
// non-numeric IID returns the zero value.
func SplitGitLabIssuePath(escapedPath string) GitLabIssuePathParts {
	trimmed := strings.Trim(path.Clean("/"+escapedPath), "/")
	parts := strings.Split(trimmed, "/")
	for i := 0; i+2 < len(parts); i++ {
		if parts[i] != "-" || (parts[i+1] != "issues" && parts[i+1] != "work_items") {
			continue
		}
		kind := parts[i+1]
		iid := parts[i+2]
		if _, err := strconv.Atoi(iid); err != nil {
			return GitLabIssuePathParts{}
		}
		projectParts := parts[:i]
		for index, part := range projectParts {
			projectParts[index], _ = url.PathUnescape(part)
		}
		return GitLabIssuePathParts{Project: strings.Join(projectParts, "/"), IID: iid, Kind: kind}
	}
	return GitLabIssuePathParts{}
}
