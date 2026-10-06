package summary

import (
	app "issueops/internal/application/selfverify"
	selfverify "issueops/internal/contract/selfverify"

	"testing"

	"issueops/internal/contract/failurecause"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func TestSummarizeSelfVerificationAddsOrthogonalFailureCause(t *testing.T) {
	ok := app.SummarizeSelfVerification(augmentcontract.SelfAugmentResult{OK: true}, 95)
	if ok.FailureCause != failurecause.None || ok.FailureCauseEvidence == nil {
		t.Fatalf("success cause = %#v", ok)
	}
	failed := app.SummarizeSelfVerification(augmentcontract.SelfAugmentResult{Runs: []augmentcontract.SelfAugmentIteration{{Steps: []selfverify.StepResult{{Label: "probe", FailureEvidence: []failurecause.Evidence{{Cause: failurecause.Transport, Code: "framing", Source: "mcp"}}}}}}}, 95)
	if failed.FailureCause != failurecause.Transport {
		t.Fatalf("cause = %s", failed.FailureCause)
	}
}
