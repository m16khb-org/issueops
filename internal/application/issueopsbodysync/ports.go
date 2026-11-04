// Package issueopsbodysync 패키지는 원격 이슈·PR·MR 산출물 본문을 기록된 기준선과
// 비교해 병합하고, 확인한 경우에만 원격에 쓰는 본문 동기화 유스케이스를 담당한다.
// application 계층이며 저장소와 provider는 포트로만 접근한다.
package issueopsbodysync

import (
	"context"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// RecordTransition 타입은 Repository.Update가 읽은 레코드를 받아 갱신된 레코드를
// 돌려주는 변환 함수다.
type RecordTransition func(model.Record) (model.Record, error)

// Repository 타입은 동기화가 쓰는 IssueOps 레코드 저장소다. Read는 id의 레코드를
// 읽고, Update는 id의 레코드에 변환을 적용해 갱신된 레코드를 돌려준다.
type Repository interface {
	Read(context.Context, string) (model.Record, error)
	Update(context.Context, string, RecordTransition) (model.Record, error)
}

type authority interface {
	Authorize(context.Context, model.Record, model.Actor) error
}

// Provider 타입은 원격 산출물 본문을 읽고 교체하며 자식 이슈 계층을 확인하는
// provider 포트다. Name은 결과에 기록되는 provider 이름이다.
type Provider interface {
	Name() string
	port.IssueProviderArtifactBodyReader
	port.IssueProviderArtifactBodyReplacer
	port.IssueProviderChildHierarchyVerifier
}
