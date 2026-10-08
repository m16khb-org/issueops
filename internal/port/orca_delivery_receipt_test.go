package port

import "testing"

// omp runs Orca's injected dispatch like codex and claude: Orca recognizes
// omp terminals, so the receipt carries no terminal-send prompt receipt.
func TestValidateExecutionOrcaDeliveryReceiptTreatsOmpAsInjectedDispatch(t *testing.T) {
	const dispatchID = "11111111-1111-4111-8111-111111111111"
	receipt := ExecutionOrcaIntentReceipt{
		TaskID: "task-1", DispatchID: "dispatch-1", RequestID: dispatchID,
		TerminalPTYID: "pty-1", TerminalHandle: "term-1",
	}
	expected := OrcaDeliveryReceiptExpectation{
		Host: "omp", TaskID: "task-1", TerminalPTYID: "pty-1", TerminalHandle: "term-1", DispatchRequestID: dispatchID,
	}
	if err := ValidateExecutionOrcaDeliveryReceipt(receipt, expected); err != nil {
		t.Fatalf("omp injected dispatch receipt rejected: %v", err)
	}
	baseline := uint64(0)
	receipt.PromptReceipt = &OrcaPromptReceipt{
		RequestID: "22222222-2222-4222-8222-222222222222", Stages: []string{"input_accepted"},
		Provider: "omo", ProcessIncarnation: "process-1", Generation: 1, BaselineWorkingSequence: &baseline,
	}
	if err := ValidateExecutionOrcaDeliveryReceipt(receipt, expected); err == nil {
		t.Fatal("omp injected dispatch accepted an unexpected prompt receipt")
	}
}
