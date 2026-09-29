package basiccli

import (
	doctorlg "issueops/internal/adapter/doctor"
)

// production wiring과 같은 loop gate 조회를 설치한다.
func init() {
	doctorlg.RepoGateSummaryFor = testLoopRepoGateSummaryFor
	doctorlg.LoopStateRoot = testLoopStateRoot
}
