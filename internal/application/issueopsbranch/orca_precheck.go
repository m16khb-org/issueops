package issueopsbranch

import (
	domain "issueops/internal/domain/issueops"
	port "issueops/internal/port/issueopsbranch"
)

type OrcaBranchPrecheck struct{ Observations port.OrcaBranchObservations }

func (check OrcaBranchPrecheck) Check(id, branch string) (string, error) {
	record, err := check.Observations.ReadRecord(id)
	if err != nil {
		return "orca_branch_precheck_failed", err
	}
	scopes, err := domain.OrcaBranchScopes(branch)
	if err != nil {
		return "orca_branch_name_taken", err
	}
	for _, scope := range scopes {
		oid, present := check.Observations.RefOID(record.Repo, scope.Ref)
		if !present {
			continue
		}
		if err := domain.ValidateOrcaBranchRef(record.BranchPrepare, scope, oid); err != nil {
			return "orca_branch_name_taken", err
		}
	}
	return "", nil
}
