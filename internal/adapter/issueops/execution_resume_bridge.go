package issueops

import (
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ExecutionResumeArtifactsReceipt struct {
	ClaimTokenPath      string
	IssueBodySHA256     string
	ContextPacketPath   string
	ContextPacketSHA256 string
	OwnerPromptPath     string
	OwnerPromptSHA256   string
}

type ExecutionResumeIntentState struct {
	Record             issueops.IssueOpsRecord
	RecordRaw          []byte
	IntentRaw          []byte
	OperationID        string
	Stage              port.ExecutionOrcaIntentStage
	InvocationState    string
	InvocationAttempts int
	Pending            bool
}

func ReadExecutionResumeArtifacts(record issueops.IssueOpsRecord) (ExecutionResumeArtifactsReceipt, error) {
	artifacts, err := readExecutionResumeArtifacts(record)
	if err != nil {
		return ExecutionResumeArtifactsReceipt{}, err
	}
	return executionResumeArtifactsReceipt(artifacts), nil
}

func NewExecutionResumeOperationID() (string, error) { return newExecutionOperationID() }

func ExecutionResumeIntentRequest(expected ExecutionResumeIntentState) (port.ExecutionOrcaIntentRequest, error) {
	payload, err := executionResumeIntentPayload(expected)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentExpectedRecord(expected.Record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return executionOrcaIntentRequest(expected.Record, payload)
}

func executionResumeArtifactsReceipt(artifacts executionResumeArtifacts) ExecutionResumeArtifactsReceipt {
	return ExecutionResumeArtifactsReceipt{ClaimTokenPath: artifacts.claimTokenPath, IssueBodySHA256: artifacts.issueBodySHA256, ContextPacketPath: artifacts.packetPath, ContextPacketSHA256: artifacts.packetSHA256, OwnerPromptPath: artifacts.promptPath, OwnerPromptSHA256: artifacts.promptSHA256}
}

func executionResumeIntentPayload(expected ExecutionResumeIntentState) (externalOrcaIntentPayload, error) {
	return preparationIntentCodec.Decode(expected.OperationID, expected.IntentRaw)
}
