package issueops

import (
	"syscall"
	"time"

	cleanupapp "issueops/internal/application/issueopscleanup"
	"issueops/internal/port"
)

type CleanupProcessController struct{ deps CleanupProcessDeps }

func NewCleanupProcessController(deps CleanupProcessDeps) CleanupProcessController {
	return CleanupProcessController{deps: deps.withDefaults()}
}
func (c CleanupProcessController) Observe(root string) (port.CleanupWorkspaceOccupancy, error) {
	return c.deps.Observe(root)
}
func (c CleanupProcessController) RequesterPID() int        { return c.deps.SelfPID }
func (c CleanupProcessController) Hangup(pid int)           { _ = c.deps.Signal(pid, syscall.SIGHUP) }
func (c CleanupProcessController) Terminate(pid int)        { _ = c.deps.Signal(pid, syscall.SIGTERM) }
func (c CleanupProcessController) Kill(pid int)             { _ = c.deps.Signal(pid, syscall.SIGKILL) }
func (c CleanupProcessController) Wait(delay time.Duration) { c.deps.Sleep(delay) }

func NewCleanupWorkspaceCleaner(deps CleanupProcessDeps, orca port.CleanupOrcaTerminals) cleanupapp.WorkspaceCleaner {
	deps = deps.withDefaults()
	return cleanupapp.WorkspaceCleaner{Processes: NewCleanupProcessController(deps), Orca: orca, SamePath: cleanupStrictSamePath, PaneKey: deps.Getenv("ORCA_PANE_KEY"), Handle: deps.Getenv("ORCA_TERMINAL_HANDLE")}
}
