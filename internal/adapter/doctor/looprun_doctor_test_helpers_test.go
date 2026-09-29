package doctor

// production wiring과 같은 loop gate 조회를 설치한다.
func init() {
	RepoGateSummaryFor = testLoopRepoGateSummaryFor
	LoopStateRoot = testLoopStateRoot
}
