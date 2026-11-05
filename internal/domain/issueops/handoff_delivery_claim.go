package issueops

import (
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

// HandoffDeliveryClaimObservable 함수는 claim 결과가 활성 holder를 가진 성공 결과일 때만 true다.
func HandoffDeliveryClaimObservable(result issueopscontract.ExecutionResult) bool {
	return result.OK && result.Execution.Lease.Status == issueopscontract.LeaseStatusActive && result.Execution.Lease.Holder != nil
}

// OwnerClaimHandoffDeliveryObservation 함수는 claim 결과가 관측 가능할 때 직전 generation의
// manual handoff 관찰에 owner claim을 기록한 사본을 돌려준다. 관측할 수 없거나 후보가 없으면 nil이고, 후보가
// 둘 이상이거나 증거가 유효하지 않으면 오류다.
func OwnerClaimHandoffDeliveryObservation(result issueopscontract.ExecutionResult, observations []issueopscontract.HandoffDeliveryObservation) (*issueopscontract.HandoffDeliveryObservation, error) {
	if !HandoffDeliveryClaimObservable(result) {
		return nil, nil
	}
	return manualOwnerClaimObservation(result, observations)
}

func manualOwnerClaimObservation(result issueopscontract.ExecutionResult, observations []issueopscontract.HandoffDeliveryObservation) (*issueopscontract.HandoffDeliveryObservation, error) {
	candidates := manualClaimCandidates(result, observations)
	folded, decisions := FoldHandoffDeliveryObservations(candidates)
	for _, decision := range decisions {
		if !decision.Accepted {
			return nil, fmt.Errorf("manual handoff delivery claim evidence is invalid: %s", strings.Join(decision.RejectReasons, "; "))
		}
	}
	if len(folded) == 0 {
		return nil, nil
	}
	if len(folded) != 1 {
		return nil, fmt.Errorf("manual handoff delivery claim evidence is ambiguous")
	}
	holderProcess := result.Execution.Lease.Holder.SessionProcess
	for _, observation := range folded {
		if manualClaimFromHolderProcess(observation, holderProcess) {
			return claimedHandoffDeliveryObservation(observation, result.Execution.Lease)
		}
	}
	return nil, nil
}

// manualClaimCandidates는 직전 generation에서 현재 holder host로 넘긴 manual lineage 관찰만 고른다.
func manualClaimCandidates(result issueopscontract.ExecutionResult, observations []issueopscontract.HandoffDeliveryObservation) []issueopscontract.HandoffDeliveryObservation {
	candidates := make([]issueopscontract.HandoffDeliveryObservation, 0, 1)
	for _, observation := range observations {
		if observation.LifecycleID == result.ID && result.Execution.Lease.Generation > 1 &&
			observation.SourceGeneration == result.Execution.Lease.Generation-1 &&
			strings.HasPrefix(observation.LineageID, handoffDeliveryManualLineagePrefix) && observation.ExpectedOwnerHost == result.Execution.Lease.Holder.Host {
			candidates = append(candidates, observation)
		}
	}
	return candidates
}

// manualClaimFromHolderProcess는 입력 또는 네이티브 턴이 관찰됐고 대상 프로세스가 holder 프로세스와 같은지 확인한다.
func manualClaimFromHolderProcess(observation issueopscontract.HandoffDeliveryObservation, holderProcess *issueopscontract.NativeProcessReceipt) bool {
	process := observation.Target.Process
	accepted := observation.InputAccepted.Status == issueopscontract.HandoffDeliveryStateObserved ||
		observation.NativeTurnObserved.Status == issueopscontract.HandoffDeliveryStateObserved
	return accepted && holderProcess != nil && process != nil && process.PID == holderProcess.PID &&
		process.StartedAt == holderProcess.StartedAt && process.Executable == holderProcess.Executable
}

func claimedHandoffDeliveryObservation(observation issueopscontract.HandoffDeliveryObservation, lease issueopscontract.WriteLease) (*issueopscontract.HandoffDeliveryObservation, error) {
	holder := *lease.Holder
	claimedAt := strings.TrimSpace(lease.ClaimedAt)
	observation.UpdatedAt = claimedAt
	observation.OwnerActor = &holder
	observation.OwnerClaimed = issueopscontract.HandoffDeliveryState{
		Status: issueopscontract.HandoffDeliveryStateObserved, ObservedAt: claimedAt,
		Evidence: issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim,
	}
	observation.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
		Claimed: true, Generation: lease.Generation, Actor: holder, ClaimedAt: claimedAt,
	}
	if err := ValidateHandoffDeliveryObservation(observation); err != nil {
		return nil, err
	}
	return &observation, nil
}
