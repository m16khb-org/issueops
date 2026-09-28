package issueopscycle

import gatescontract "issueops/internal/contract/gates"

type GateLedgerReadiness struct {
	Discover func(root string) ([]string, error)
	Check    func(gatescontract.CheckRequest) (gatescontract.CheckResult, error)
}
