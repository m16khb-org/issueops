package issueopscli

import (
	"context"
	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/preflight"
	modeswitch "issueops/internal/application/issueopsmodeswitch"
	model "issueops/internal/contract/issueops"
	"time"
)

func testModeSwitcher() func(context.Context, string, model.ExecutionSwitchModeRequest) (model.ExecutionSwitchModeResult, error) {
	workspace := core.ModeSwitchWorkspace{Git: func(dir string, args ...string) (int, string) {
		code, stdout, stderr := preflight.GitCmd(dir, args...)
		if code != 0 && stderr != "" {
			return code, stderr
		}
		return code, stdout
	}}
	return func(ctx context.Context, root string, req model.ExecutionSwitchModeRequest) (model.ExecutionSwitchModeResult, error) {
		return (modeswitch.Service{Records: core.CycleRecordStore{StateRoot: root}, Workspace: workspace, Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) }}).Run(ctx, req)
	}
}
