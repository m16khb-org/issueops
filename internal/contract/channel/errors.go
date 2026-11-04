package channel

import "errors"

// ErrFromRequired 변수는 channel send 요청에 발신자(From)가 비어 있을 때 돌려주는 오류다.
var ErrFromRequired = errors.New("from_required")
