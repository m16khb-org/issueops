package verifyloop

import (
	selfverify "issueops/internal/contract/selfverify"
	selfverifydomain "issueops/internal/domain/selfverify"

	"fmt"
	"os"
	"path/filepath"
	"time"

	"issueops/cmd/issueops/commandstep"
	"issueops/cmd/issueops/selfworkflow/progress"
	application "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
)

type Deps struct {
	IssueOpsRoot func() string
	StepDeps     application.SelfVerifyStepDeps
	FailedStep   func(string, error) selfverify.StepResult
	PrintStep    func(selfverify.StepResult)
	Printf       func(string, ...any) (int, error)
}

type Request struct {
	BaseSeed        int64
	TargetScore     float64
	Verbose         bool
	Reporter        *progress.SelfVerifyProgressReporter
	CollectAllSteps bool
}

func SelfVerify(request Request, deps Deps) (augmentcontract.SelfAugmentResult, error) {
	deps = deps.withDefaults()
	var reporter application.ProgressReporter
	if request.Reporter != nil {
		reporter = request.Reporter
	}
	return application.ExecuteLoop(application.LoopRequest{
		BaseSeed: request.BaseSeed, TargetScore: request.TargetScore,
		Verbose: request.Verbose, Reporter: reporter, CollectAllSteps: request.CollectAllSteps,
	}, application.LoopDeps{
		IssueOpsRoot:   deps.IssueOpsRoot,
		StepDeps:       deps.StepDeps,
		FailedStep:     deps.FailedStep,
		PrintStep:      deps.PrintStep,
		Printf:         deps.Printf,
		Summarize:      application.SummarizeSelfVerification,
		MkdirTemp:      func() (string, error) { return os.MkdirTemp("", "issueops-self-verify-*") },
		RemoveAll:      os.RemoveAll,
		TempBinaryPath: func(dir string) string { return filepath.Join(dir, "issueops") },
		Now:            time.Now,
	})
}

func (deps Deps) withDefaults() Deps {
	if deps.IssueOpsRoot == nil {
		deps.IssueOpsRoot = func() string { return "." }
	}
	if deps.FailedStep == nil {
		deps.FailedStep = selfverifydomain.FailedStep
	}
	if deps.PrintStep == nil {
		deps.PrintStep = commandstep.PrintStep
	}
	if deps.Printf == nil {
		deps.Printf = fmt.Printf
	}
	return deps
}
