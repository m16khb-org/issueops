// Package issueopsremote 패키지는 원격 provider(이슈, PR/MR)에 쓰는 유스케이스를 조율하는 응용 계층이다.
// 이슈·자식 이슈·PR/MR 생성과 복구, 댓글·라벨·담당자·마일스톤·제목 변경, 완료 반영과 병합 검증을 다루며
// 각 쓰기는 durable 의도 기록과 행위자 권한 검사를 거친다. 도메인 규칙은 domain 계층에,
// provider 호출은 port 인터페이스 뒤에 두고 이 패키지는 둘을 잇는다.
package issueopsremote

import (
	"context"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

// PreparationObserver 타입은 PR/MR 생성 준비에 필요한 기록, 작업 트리 지문, HEAD 커밋을 관찰하는 포트다.
type PreparationObserver interface {
	Read(context.Context, string) (model.Record, error)
	Fingerprint(context.Context, model.Record) string
	Head(context.Context, model.Record) string
}

// PublicationStore 타입은 게시 의도의 payload와 기록 스냅샷을 읽고 한 트랜잭션 안에서 영속화하는 포트다.
type PublicationStore interface {
	WithinTransaction(context.Context, string, func(context.Context) error) error
	Read(context.Context, string) (model.Record, error)
	ReadPayload(context.Context, string) (contract.IntentPayload, error)
	RecordSnapshot(context.Context, string) (contract.RecordSnapshot, error)
	PayloadRaw(context.Context, string) ([]byte, error)
	DecodeSnapshot(contract.Intent) (model.Record, contract.IntentPayload, error)
	Persist(context.Context, model.Record, contract.IntentMutation) (model.Record, error)
}

// PublicationEnvironment 타입은 게시 저널이 쓰는 경로 비교, 시각, 작업 ID 생성을 제공하는 포트다.
type PublicationEnvironment interface {
	PathsMatch(string, string) bool
	Timestamp() string
	NewOperationID() (string, error)
}

// RecordTransition 타입은 기록 한 건을 다음 상태로 바꾸는 함수다. 저장소 갱신 안에서 호출된다.
type RecordTransition func(model.Record) (model.Record, error)

// RecordStore 타입은 기록을 읽고 transition을 적용해 원자적으로 갱신하는 포트다.
type RecordStore interface {
	Update(context.Context, string, RecordTransition) (model.Record, error)
}

type authority interface {
	Authorize(context.Context, model.Record, model.Actor) error
}
