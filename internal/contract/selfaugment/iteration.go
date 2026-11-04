package selfaugment

import selfverify "issueops/internal/contract/selfverify"

type Iteration struct {
	Iteration int                     `json:"iteration"`
	Seed      int64                   `json:"seed"`
	Steps     []selfverify.StepResult `json:"steps"`
}
