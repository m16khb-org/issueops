package issueopsapp

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	cmuxadapter "issueops/internal/adapter/cmux"
	"issueops/internal/adapter/hostprotocol"
	issueopsadapter "issueops/internal/adapter/issueops"
	app "issueops/internal/application/issueopscmux"
	cmuxcontract "issueops/internal/contract/cmux"
	issueopscontract "issueops/internal/contract/issueops"
	port "issueops/internal/port/cmux"
)

type cmuxHandoffDependencies struct {
	Client                 port.Client
	Now                    func() time.Time
	Getwd                  func() (string, error)
	GitTop                 func(string) (string, error)
	ReadPrompt             func(string, string, string) ([]byte, error)
	ValidateHostExecutable func(string) error
	Prepare                func(cmuxcontract.ArtifactRequest) (cmuxcontract.PreparedLauncher, error)
	AwaitReceipt           func(context.Context, cmuxcontract.PreparedLauncher, cmuxcontract.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error)
	Cleanup                func(cmuxcontract.PreparedLauncher) error
}

func issueOpsCmuxHandoffHandler(ctx context.Context, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest) (issueopscontract.ExecutionCmuxHandoffResult, error) {
	return newCmuxHandoffService(stateRoot, cmuxHandoffDependencies{Client: cmuxadapter.Client{Runner: cmuxadapter.ExecRunner{}, ObserveEndpoint: cmuxadapter.ObserveEndpoint, UID: os.Getuid()}, Now: time.Now, GitTop: cmuxadapter.GitTop, ValidateHostExecutable: cmuxadapter.ValidateHostExecutable}).Handoff(ctx, stateRoot, request)
}
func newCmuxHandoffService(stateRoot string, deps cmuxHandoffDependencies) app.Service {
	if deps.Getwd == nil {
		deps.Getwd = os.Getwd
	}
	if deps.ReadPrompt == nil {
		deps.ReadPrompt = cmuxadapter.ReadPrompt
	}
	if deps.Prepare == nil {
		deps.Prepare = func(req cmuxcontract.ArtifactRequest) (cmuxcontract.PreparedLauncher, error) {
			extra, err := roleAgentArgs(context.Background(), stateRoot, req.Host, req.CWD)
			if err != nil {
				return cmuxcontract.PreparedLauncher{}, fmt.Errorf("resolve role agents for the cmux owner session: %w", err)
			}
			return cmuxadapter.PrepareLauncher(req, func(host, executable, model, effort, prompt string) ([]string, error) {
				return hostprotocol.BuildInteractiveArgv(host, executable, model, effort, prompt, extra)
			})
		}
	}
	if deps.AwaitReceipt == nil {
		deps.AwaitReceipt = func(ctx context.Context, prepared cmuxcontract.PreparedLauncher, expected cmuxcontract.BootstrapExpectation) (issueopscontract.NativeProcessReceipt, error) {
			return cmuxadapter.AwaitBootstrapReceipt(ctx, prepared, expected, issueopsadapter.ObserveNativeProcessReceipt)
		}
	}
	if deps.Cleanup == nil {
		deps.Cleanup = cmuxadapter.CleanupLauncher
	}
	return app.Service{Client: deps.Client, Now: deps.Now, Roots: port.RootEnvironment{Read: issueopsadapter.ReadIssueOps, Directory: cmuxadapter.ObserveDirectory, Getwd: deps.Getwd, GitTop: deps.GitTop}, ReadPrompt: deps.ReadPrompt, ValidateHostExecutable: deps.ValidateHostExecutable,
		ValidateHostProfile: func(host, executable, model, effort string) error {
			_, err := hostprotocol.BuildInteractiveArgv(host, executable, model, effort, "", nil)
			return err
		},
		Prepare: deps.Prepare, AwaitReceipt: deps.AwaitReceipt, Cleanup: deps.Cleanup, ArtifactRoot: filepath.Join(stateRoot, "cmux-handoff"), SamePath: cmuxadapter.SamePath, ReadAudit: newHandoffDeliveryAudit(stateRoot).Read, Observe: newHandoffDeliveryService(stateRoot).ObserveManualSnapshot}
}
