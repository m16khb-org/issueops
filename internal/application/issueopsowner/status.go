// Package issueopsowner 패키지는 IssueOps execution 레코드의 소유 관계를 다룬다.
// workspace 위치를 정하고 봉인 아티팩트를 materialize하며, writer가 없는 execution의
// 복구 명령을 안내한다. application 계층이다.
package issueopsowner

import (
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
)

// ExecutionStatus 타입은 ReadRecord로 읽은 레코드에서 execution 상태를 조회하는
// 읽기 전용 서비스다.
type ExecutionStatus struct {
	ReadRecord func(string) (issueops.Record, error)
}

func (s ExecutionStatus) Read(id string) (issueops.ExecutionResult, error) {
	record, err := s.ReadRecord(id)
	if err != nil {
		return issueops.ExecutionResult{OK: false, ID: id}, err
	}
	if record.Execution == nil {
		return issueops.ExecutionResult{OK: false, ID: id}, fmt.Errorf("IssueOps execution v1 is not prepared")
	}
	result := issueops.ExecutionResult{OK: true, ID: record.ID, Execution: *record.Execution}
	if record.Execution.Completion == nil {
		result.NextCommand = WriterlessCommand(record)
	}
	return result, nil
}

// WriterlessCommand 함수는 writer가 없는 execution을 되살리려고 다음에 실행할 명령
// (교체 미리보기, claim, 교체 확정 미리보기 중 하나)을 돌려준다. execution이 없거나
// 복구가 필요 없는 상태면 빈 문자열이다.
func WriterlessCommand(record issueops.Record) string {
	if record.Execution == nil {
		return ""
	}
	lease := record.Execution.Lease
	switch leasedomain.DecideWriterlessRecovery(string(lease.Status)) {
	case leasedomain.RecoveryReplacePreview:
		return domain.ReplacementPreviewCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryClaim:
		return domain.ReplacementClaimCommand(record.ID, lease.Generation)
	case leasedomain.RecoveryFinalizePreview:
		return domain.ReplacementFinalizePreviewCommand(record.ID, lease.Generation)
	}
	return ""
}
