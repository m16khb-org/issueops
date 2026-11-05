package augmentplan

import (
	statecontract "issueops/internal/contract/state"
)

// issueops state 접근은 composition root가 설치한다. transport는 state를 어디에
// 어떻게 저장하는지 알지 않는다.
var (
	StateList func() (statecontract.ListResult, error)
	StateRead func(key string) (statecontract.Result, error)
)
