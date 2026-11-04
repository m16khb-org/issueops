package issueopscycle

import gatescontract "issueops/internal/contract/gates"

// GateLedgerReadiness lets readiness include task gate ledgers: Discover lists the ledger files under a root
// and Check evaluates them.
type GateLedgerReadiness struct {
	Discover func(root string) ([]string, error)
	Check    func(gatescontract.CheckRequest) (gatescontract.CheckResult, error)
}
