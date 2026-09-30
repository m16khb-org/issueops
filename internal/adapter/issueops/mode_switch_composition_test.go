package issueops

import (
	issueopscontract "issueops/internal/contract/issueops"
)

import (
	"context"
	"issueops/internal/adapter/preflight"
	modeswitch "issueops/internal/application/issueopsmodeswitch"
)

type ExecutionSwitchModeDependencies struct {
	Git func(string, ...string) (int, string)
}

func SwitchExecutionMode(ctx context.Context, root string, req issueopscontract.ExecutionSwitchModeRequest, deps ExecutionSwitchModeDependencies) (issueopscontract.ExecutionSwitchModeResult, error) {
	if deps.Git == nil {
		deps.Git = func(dir string, args ...string) (int, string) {
			code, out, err := preflight.GitCmd(dir, args...)
			if code != 0 && err != "" {
				return code, err
			}
			return code, out
		}
	}
	return (modeswitch.Service{Records: CycleRecordStore{StateRoot: root}, Workspace: ModeSwitchWorkspace(deps), Now: func() string { return executionNow(nil) }}).Run(ctx, req)
}
