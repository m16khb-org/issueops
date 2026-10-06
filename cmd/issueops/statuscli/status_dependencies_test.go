package statuscli

import (
	"os"

	inspect "issueops/internal/contract/inspect"
)

// Deps provides observations for the retained status CLI integration fixtures.
type Deps struct {
	IssueOpsRoot   func() string
	ResolveTarget  func(string) string
	Version        string
	InspectHarness func(string) inspect.InspectInfo
}

var deps = defaultDeps()

func defaultDeps() Deps {
	return Deps{
		IssueOpsRoot:   defaultIssueOpsRoot,
		ResolveTarget:  defaultResolveTarget,
		Version:        "dev",
		InspectHarness: func(string) inspect.InspectInfo { return inspect.InspectInfo{} },
	}
}

func defaultIssueOpsRoot() string {
	if root := os.Getenv("ISSUEOPS_ROOT"); root != "" {
		return root
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func defaultResolveTarget(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}
