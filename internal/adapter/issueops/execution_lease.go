package issueops

import (
	"context"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	"strconv"
	"strings"
)

func StatusExecution(stateRoot, id string) (ExecutionResult, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return ExecutionResult{OK: false, ID: id}, err
	}
	if record.Execution == nil {
		return ExecutionResult{OK: false, ID: id}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	result := executionResult(record)
	if record.Execution.Completion == nil {
		result.NextCommand = executionWriterAbsentRecoveryCommand(record)
	}
	return result, nil
}

// ExecutionWriterAbsentRecoveryCommand는 writer 없는 lease의 회복 명령을
// 노출한다. 본체는 아래 한 곳뿐이며, 단계 분류가 같은 문자열을 다시 만들지
// 않도록 감싸기만 한다.
func ExecutionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	return executionWriterAbsentRecoveryCommand(record)
}

func executionWriterAbsentRecoveryCommand(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	lease := record.Execution.Lease
	identityComplete := false
	if lease.Status == issueops.LeaseStatusClaimable && record.Execution.Mode == issueops.ExecutionModeOrca {
		identityComplete = completeOrcaArtifactIdentity(record.Execution.Orca)
	}
	switch leasedomain.DecideWriterlessRecovery(string(lease.Status), string(record.Execution.Mode), identityComplete) {
	case leasedomain.RecoveryReplacePreview:
		return executionReplacementPreviewCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryResume:
		return domain.ReplacementResumeCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryDirectClaim:
		return domain.ReplacementClaimCommand(record.ID, lease.Generation, claimTokenPath(record))
	case leasedomain.RecoveryFinalizePreview:
		return "issueops execution replace --id " + quoteExecutionOwnerArg(record.ID) +
			" --expected-generation " + strconv.FormatUint(lease.Generation, 10) + " --finalize-preview"
	}
	return ""
}

func executionReplacementPreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}

// executionOwnerReseal은 replacement가 재봉인한 owner artifact의 정체다.
// direct 모드 사이클에서는 모든 필드가 빈 값으로 남는다(재봉인 대상이 아니다).
type executionOwnerReseal struct {
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

// resealOwnerContextForReplacement는 replacement generation과 새 claim token이
// 반영된 레코드를 기준으로 owner packet과 prompt를 다시 봉인한다.
//
// 봉인은 원래 prepare의 worktree 단계에서 단 1회 일어났고 finalize와 reseed는
// lease만 회전시켰다. 그 결과 새 세대 owner가 lease_generation과
// claim_token_file이 어긋난 낡은 packet을 읽었고, 봉인 이후 이슈 본문이
// 정당하게 개정되면 재봉인 수단이 없어 claim이 영구 거부됐다. 여기서 현재
// 이슈 본문을 다시 읽어 봉인하면 두 문제가 함께 해소된다.
//
// 원격 읽기 실패는 통과가 아니라 거부다: 낡은 packet으로 owner를 띄우는 것보다
// replacement를 멈추는 편이 안전하고 재시도로 해소된다.
func resealOwnerContextForReplacement(ctx context.Context, stateRoot string, record issueops.IssueOpsRecord, readIssue ExecutionIssueSnapshotReadFunc) (executionOwnerReseal, error) {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Orca == nil {
		return executionOwnerReseal{}, nil
	}
	if strings.TrimSpace(record.PlanPath) == "" {
		return executionOwnerReseal{}, newPlanArtifactRequiredError(record, false)
	}
	stagedPlan, err := RequireStagedExecutionOwnerPlan(stateRoot, record)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	if readIssue == nil {
		return executionOwnerReseal{}, fmt.Errorf("replacement cannot reseal the owner context without a remote issue reader")
	}
	snapshot, err := readExecutionOwnerSnapshot(ctx, record, readIssue)
	if err != nil {
		return executionOwnerReseal{}, fmt.Errorf("replacement stopped because the remote issue could not be read for resealing: %w", err)
	}
	plan, manifest, err := materializeExecutionOwnerArtifacts(stateRoot, record)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	if plan.Path != record.PlanPath || plan.Digest != stagedPlan.Digest {
		return executionOwnerReseal{}, newPlanArtifactRequiredError(record, false)
	}
	binding := record.Execution.Orca
	artifacts, err := buildExecutionOwnerArtifacts(record, ExecutionPrepareRequest{
		ID: record.ID, Mode: string(issueops.ExecutionModeOrca), OwnerHost: binding.OwnerHost,
		OwnerModel: binding.OwnerModel, OwnerEffort: binding.OwnerEffort,
	}, snapshot, manifest)
	if err != nil {
		return executionOwnerReseal{}, err
	}
	return executionOwnerReseal{
		issueBodySHA256: snapshot.issue.BodySHA256,
		packetPath:      artifacts.packetPath, packetSHA256: artifacts.packetSHA256,
		promptPath: artifacts.promptPath, promptSHA256: artifacts.promptSHA256,
	}, nil
}

func executionRecordAtGeneration(stateRoot, id string, generation uint64) (issueops.IssueOpsRecord, error) {
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, err
	}
	return record, domain.ValidateReplacementGeneration(record, generation, false)
}

func executionResult(record issueops.IssueOpsRecord) ExecutionResult {
	return ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
}
