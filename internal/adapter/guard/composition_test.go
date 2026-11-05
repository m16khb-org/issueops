package guard

import (
	guardapp "issueops/internal/application/guard"
	guardcontract "issueops/internal/contract/guard"
)

func GuardCheck(req guardcontract.CheckRequest) guardcontract.CheckResult {
	return (guardapp.Service{Source: Source{}}).Check(req)
}
