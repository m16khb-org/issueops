package issueopslease

import "context"

// noRoleAgents resolves no role-agent arguments, as for an omo owner.
func noRoleAgents(context.Context, string, string) ([]string, error) { return nil, nil }
