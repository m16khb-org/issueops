package statuscli

import (
	"os"

	"issueops/cmd/issueops/daemoncli"
	inspect "issueops/internal/contract/inspect"
)

// Deps provides observations for the retained status CLI integration fixtures.
type Deps struct {
	IssueOpsRoot      func() string
	ResolveTarget     func(string) string
	Version           string
	InspectHarness    func(string) inspect.InspectInfo
	CheckDaemonStatus func() daemoncli.Status
}

var deps = defaultDeps()

// Configure changes only this test package's fixture dependencies.
func Configure(d Deps) { deps = d }

func defaultDeps() Deps {
	return Deps{
		IssueOpsRoot:      defaultIssueOpsRoot,
		ResolveTarget:     defaultResolveTarget,
		Version:           "dev",
		InspectHarness:    func(string) inspect.InspectInfo { return inspect.InspectInfo{} },
		CheckDaemonStatus: daemoncli.CheckDaemonStatus,
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
