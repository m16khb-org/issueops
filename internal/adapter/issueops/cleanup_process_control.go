package issueops

import (
	"syscall"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/port"
)

type cleanupProcessController struct{ deps CleanupProcessDeps }

func newCleanupProcessController(deps CleanupProcessDeps) cleanupProcessController {
	return cleanupProcessController{deps: deps.withDefaults()}
}
func (c cleanupProcessController) Observe(root string) (port.CleanupWorkspaceOccupancy, error) {
	return c.deps.Observe(root)
}
func (c cleanupProcessController) RequesterPID() int        { return c.deps.SelfPID }
func (c cleanupProcessController) Hangup(pid int)           { _ = c.deps.Signal(pid, syscall.SIGHUP) }
func (c cleanupProcessController) Terminate(pid int)        { _ = c.deps.Signal(pid, syscall.SIGTERM) }
func (c cleanupProcessController) Kill(pid int)             { _ = c.deps.Signal(pid, syscall.SIGKILL) }
func (c cleanupProcessController) Wait(delay time.Duration) { c.deps.Sleep(delay) }

func newCleanupWorkspaceCleaner(deps CleanupProcessDeps) cleanupapp.WorkspaceCleaner {
	deps = deps.withDefaults()
	return cleanupapp.WorkspaceCleaner{Processes: newCleanupProcessController(deps), SamePath: cleanupStrictSamePath}
}
