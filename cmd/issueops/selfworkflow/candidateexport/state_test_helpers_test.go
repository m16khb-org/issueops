package candidateexport

import (
	"context"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification"
	statecontract "issueops/internal/contract/state"
)

// production wiring과 같은 state store를 설치한다. fitness graph는 test import를
// 수집하지 않으므로 여기서는 concrete를 써도 된다.
func init() {
	ObserveSource = verification.CandidateSource
	StateDir = statestore.StateDir
	StateWrite = func(key, content string) (statecontract.StateResult, error) {
		return statestore.NewService().Write(context.Background(), key, content)
	}
}
