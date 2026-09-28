package issueops

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

const (
	orcaIntentNotInvoked = "not_invoked_proven"
	orcaIntentUnknown    = "unknown"

	orcaIntentPurposePrepare = "prepare"
	orcaIntentPurposeResume  = "resume"
)

type externalOrcaLaunchIdentity = preparationcontract.LaunchIdentity
type externalOrcaIntentPayload = preparationcontract.Intent

var preparationIntentCodec preparationcontract.IntentCodec

func validateOrcaIntentExpectedRecord(record issueops.IssueOpsRecord, expected externalOrcaIntentPayload) error {
	raw, err := json.Marshal(record)
	if err != nil {
		return err
	}
	var authorityRecord preparationcontract.Record
	if err := json.Unmarshal(raw, &authorityRecord); err != nil {
		return err
	}
	if err := preparationdomain.ValidateIntentRecordAuthority(authorityRecord, expected); err != nil {
		return err
	}
	return validateOrcaIntentRecordIdentity(record, expected)
}

func readExternalOrcaIntentPayload(stateRoot, operationID string) (externalOrcaIntentPayload, error) {
	payload, err := readExternalOrcaIntentPayloadShape(stateRoot, operationID)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	if err := validateExternalOrcaIntentPayload(payload, operationID); err != nil {
		return externalOrcaIntentPayload{}, err
	}
	return payload, nil
}

func readExternalOrcaIntentPayloadShape(stateRoot, operationID string) (externalOrcaIntentPayload, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return externalOrcaIntentPayload{}, err
	}
	if !ok {
		return externalOrcaIntentPayload{}, fmt.Errorf("Orca external intent payload is missing")
	}
	return preparationIntentCodec.DecodeShape(operationID, data)
}

func validateExternalOrcaIntentPayload(payload externalOrcaIntentPayload, operationID string) error {
	return preparationIntentCodec.Validate(payload, operationID)
}

func normalizedOrcaIntentPurpose(payload externalOrcaIntentPayload) string {
	if strings.TrimSpace(payload.Purpose) == "" {
		return orcaIntentPurposePrepare
	}
	return strings.TrimSpace(payload.Purpose)
}

func BuildExecutionOrcaIntentRequest(record issueops.IssueOpsRecord, intent preparationcontract.Intent) (port.ExecutionOrcaIntentRequest, error) {
	if err := validateOrcaIntentExpectedRecord(record, intent); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	return executionOrcaIntentRequest(record, intent)
}

func executionOrcaIntentRequest(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (port.ExecutionOrcaIntentRequest, error) {
	request, err := executionOrcaIntentInspectionRequest(record, payload)
	if err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if payload.Launch == nil {
		return request, nil
	}
	token, err := readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil || tokenSHA256(token) != payload.ClaimTokenSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed claim token identity changed")
	}
	prompt, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, payload.Launch.PromptPath)
	if err != nil || digestExecutionOwnerBytes(prompt) != payload.Launch.PromptSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	packet, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, payload.Launch.ContextPacketPath)
	if err != nil || digestExecutionOwnerBytes(packet) != payload.Launch.ContextPacketSHA256 {
		return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed context packet identity changed")
	}
	request.Launch.Prompt = string(prompt)
	return request, nil
}

// executionOrcaIntentInspectionRequest는 persisted payload의 봉인 메타데이터만
// 사용해 read-only Orca 인벤토리 요청을 만든다. 삭제된 worktree의 token과
// artifact 파일은 mutation 재시도에만 필요하며, 이미 사라진 외부 자원을
// 확인하는 조회의 전제 조건이 아니다.
func executionOrcaIntentInspectionRequest(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) (port.ExecutionOrcaIntentRequest, error) {
	if err := validateExternalOrcaIntentPayload(payload, payload.OperationID); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	if err := validateOrcaIntentRecordIdentity(record, payload); err != nil {
		return port.ExecutionOrcaIntentRequest{}, err
	}
	request := port.ExecutionOrcaIntentRequest{
		Stage: intentPortStage(payload.Stage), OperationID: payload.OperationID,
		RetryRequestID: payload.OrcaRequestID, PromptRetryRequestID: payload.OrcaPromptRequestID, SourceGeneration: payload.Generation, Marker: payload.Marker,
		Workspace: intentPortWorkspaceRequest(payload.Workspace), Probe: intentPortProbeRequest(payload.Probe),
		Prepared: intentPortOrcaWorkspaceReceiptPointer(payload.Prepared), TerminalPTYID: payload.TerminalPTYID,
		RunID: payload.RunID, RunBound: payload.RunBound, TaskID: payload.TaskID,
	}
	if payload.Launch != nil {
		expectedPacketPath, expectedPromptPath := executionOwnerArtifactPaths(record)
		if !samePath(payload.Launch.PromptPath, expectedPromptPath) || !samePath(payload.Launch.ContextPacketPath, expectedPacketPath) {
			return port.ExecutionOrcaIntentRequest{}, fmt.Errorf("sealed owner artifact path changed")
		}
		request.Launch = &port.ExecutionOrcaLaunchRequest{
			PromptPath: payload.Launch.PromptPath, PromptSHA256: payload.Launch.PromptSHA256,
			ContextPacketPath: payload.Launch.ContextPacketPath, ContextPacketSHA256: payload.Launch.ContextPacketSHA256,
		}
	}
	return request, nil
}

func validateOrcaIntentRecordIdentity(record issueops.IssueOpsRecord, payload externalOrcaIntentPayload) error {
	if err := validateOrcaIntentIssueIdentity(record, payload); err != nil {
		return err
	}
	if record.ID != payload.LifecycleID || record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca ||
		!samePath(record.Execution.Workspace.SourceRoot, payload.Workspace.SourceRoot) || !samePath(record.Execution.Workspace.Root, payload.Workspace.Root) ||
		record.Execution.Workspace.Branch != payload.Workspace.Branch || record.Execution.Workspace.BaseHead != payload.Workspace.BaseHead ||
		!sameOptionalExecutionPath(record.Execution.Workspace.ParentWorktree, payload.Workspace.ParentWorktree) ||
		record.Execution.Workspace.Driver != "orca" {
		return fmt.Errorf("Orca intent record identity changed")
	}
	if payload.Prepared != nil && (!samePath(record.WorktreePath, payload.Prepared.Workspace.Root) ||
		!samePath(record.Execution.Workspace.Root, payload.Prepared.Workspace.Root) || record.Execution.Workspace.Branch != payload.Prepared.Workspace.Branch ||
		record.Execution.Workspace.BaseHead != payload.Prepared.Workspace.BaseHead ||
		!sameOptionalExecutionPath(record.Execution.Workspace.ParentWorktree, payload.Prepared.Workspace.ParentWorktree)) {
		return fmt.Errorf("Orca prepared workspace identity changed")
	}
	return nil
}

func sameOptionalExecutionPath(left, right string) bool {
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return left == right
	}
	return samePath(left, right)
}

func createOrAdoptClaimToken(record issueops.IssueOpsRecord) (string, error) {
	token, _, err := createClaimToken(record)
	if err == nil {
		return tokenSHA256(token), nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	token, err = readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil {
		return "", fmt.Errorf("recover deterministic claim token: %w", err)
	}
	return tokenSHA256(token), nil
}

func intentPortWorkspaceRequest(workspace preparationcontract.WorkspaceRequest) port.ExecutionWorkspaceRequest {
	return port.ExecutionWorkspaceRequest{
		LifecycleID: workspace.LifecycleID, SourceRoot: workspace.SourceRoot, Root: workspace.Root,
		Branch: workspace.Branch, BaseBranch: workspace.BaseBranch, BaseHead: workspace.BaseHead,
		ParentWorktree: workspace.ParentWorktree, Confirm: workspace.Confirm,
	}
}

func intentPortProbeRequest(probe preparationcontract.ProbeRequest) port.ExecutionOrcaProbeRequest {
	return port.ExecutionOrcaProbeRequest{
		Repo: probe.Repo, Host: probe.Host, Model: probe.Model, Effort: probe.Effort,
		Provider: probe.Provider, Issue: probe.Issue, Marker: probe.Marker,
	}
}

func intentPortWorkspaceReceipt(receipt preparationcontract.WorkspaceReceipt) port.ExecutionWorkspaceReceipt {
	return port.ExecutionWorkspaceReceipt{
		SourceRoot: receipt.SourceRoot, Root: receipt.Root, Branch: receipt.Branch, BaseHead: receipt.BaseHead,
		ParentWorktree: receipt.ParentWorktree, Driver: receipt.Driver, Exists: receipt.Exists,
	}
}

func intentPortOrcaWorkspaceReceiptPointer(receipt *preparationcontract.OrcaWorkspaceReceipt) *port.ExecutionOrcaWorkspaceReceipt {
	if receipt == nil {
		return nil
	}
	return &port.ExecutionOrcaWorkspaceReceipt{
		Workspace: intentPortWorkspaceReceipt(receipt.Workspace), RuntimeID: receipt.RuntimeID,
		RepoID: receipt.RepoID, WorktreeID: receipt.WorktreeID, WorktreeInstanceID: receipt.WorktreeInstanceID,
	}
}

func intentPortStage(stage preparationcontract.IntentStage) port.ExecutionOrcaIntentStage {
	return port.ExecutionOrcaIntentStage(stage)
}
