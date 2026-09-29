package guard

import (
	guardapp "issueops/internal/application/guard"
	guardcontract "issueops/internal/contract/guard"
)

func GuardCheck(req guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult {
	return (guardapp.Service{Source: Source{}}).Check(req)
}
