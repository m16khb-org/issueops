package update

import (
	updatecontract "issueops/internal/contract/update"
	installdomain "issueops/internal/domain/install"
)

type MCPProxyEffects interface {
	List() ([]updatecontract.MCPProxyProcess, error)
	Terminate(int) error
	CurrentPID() int
	SupportsOrphanTermination() bool
}

func CleanupMCPProxies(effects MCPProxyEffects, dryRun bool) (updatecontract.MCPCleanupResult, error) {
	processes, err := effects.List()
	if err != nil {
		return updatecontract.MCPCleanupResult{OK: false, DryRun: dryRun}, err
	}
	result := updatecontract.MCPCleanupResult{OK: true, DryRun: dryRun, Matched: len(processes), Processes: []updatecontract.MCPCleanupProcess{}}
	for _, process := range processes {
		item := updatecontract.MCPCleanupProcess{PID: process.PID, Command: process.Command}
		item.Action = installdomain.MCPProxyCleanupAction(proxyIdentity(process), effects.CurrentPID(), effects.SupportsOrphanTermination(), dryRun)
		if item.Action == "terminate" {
			freshProcesses, listErr := effects.List()
			if listErr != nil {
				item.Action = "skip-revalidation-error"
				result.OK = false
				result.Processes = append(result.Processes, item)
				return result, listErr
			}
			fresh, found := findMCPProxyProcess(freshProcesses, process.PID)
			if !found || !installdomain.SameMCPProxyIdentity(proxyIdentity(process), proxyIdentity(fresh)) ||
				installdomain.MCPProxyCleanupAction(proxyIdentity(fresh), effects.CurrentPID(), effects.SupportsOrphanTermination(), false) != "terminate" {
				item.Action = "skip-identity-changed"
			} else if terminateErr := effects.Terminate(process.PID); terminateErr != nil {
				item.Action = "terminate-error"
				result.OK = false
				result.Processes = append(result.Processes, item)
				return result, terminateErr
			} else {
				item.Action = "terminated"
				result.Terminated++
			}
		}
		result.Processes = append(result.Processes, item)
	}
	if dryRun {
		result.Message = "dry-run: no MCP proxy processes terminated"
	} else {
		result.Message = "MCP proxy cleanup complete"
	}
	return result, nil
}

func findMCPProxyProcess(processes []updatecontract.MCPProxyProcess, pid int) (updatecontract.MCPProxyProcess, bool) {
	for _, process := range processes {
		if process.PID == pid {
			return process, true
		}
	}
	return updatecontract.MCPProxyProcess{}, false
}

func proxyIdentity(process updatecontract.MCPProxyProcess) installdomain.MCPProxyIdentity {
	return installdomain.MCPProxyIdentity{
		PID: process.PID, ParentPID: process.ParentPID, Command: process.Command,
		StartTime: process.StartTime, Executable: process.Executable, IdentityVerified: process.IdentityVerified,
	}
}
