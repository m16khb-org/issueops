package issueopspreparation

import (
	"context"

	agentmodelcontract "issueops/internal/contract/agentmodel"
	"issueops/internal/domain/agentmodel"
)

// agentModelsFake resolves owner defaults from built-in values and returns
// fixed role-agent arguments, recording each call.
type agentModelsFake struct {
	ownerErr, argsErr error
	args              []string
	ownerCalls        []agentmodelcontract.Role
	argsCalls         []string
}

func (f *agentModelsFake) OwnerDefaults(_ context.Context, host string, role agentmodelcontract.Role, _ string) (string, string, error) {
	f.ownerCalls = append(f.ownerCalls, role)
	if f.ownerErr != nil {
		return "", "", f.ownerErr
	}
	resolution, err := agentmodel.Resolve(agentmodel.ResolveInput{Host: host, Role: role})
	return resolution.Model, resolution.Effort, err
}

func (f *agentModelsFake) RoleAgentArgs(_ context.Context, host, repo string) ([]string, error) {
	f.argsCalls = append(f.argsCalls, host+"@"+repo)
	return f.args, f.argsErr
}
