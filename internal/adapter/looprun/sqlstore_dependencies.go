package looprun

import (
	"context"

	"issueops/internal/port"
)

// StateDatabase는 이 package가 실제로 쓰는 저장소 연산만 선언한다. 구현을 고르는 것은
// composition root의 결정이고, 여기서는 필요한 만큼만 안다.
type StateDatabase interface {
	Get(bucket, id string) ([]byte, bool, error)
	Apply(ctx context.Context, mutations []port.RecordMutation) error
	WithSpan(ctx context.Context, fn func(context.Context) error) error
}
