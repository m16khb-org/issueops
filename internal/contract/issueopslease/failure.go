package issueopslease

import (
	"errors"
	"fmt"
)

// ErrExecutionNotPrepared 변수는 record에 execution v1이 아직 준비되지 않았음을 나타내는 sentinel 오류다.
var ErrExecutionNotPrepared = errors.New("IssueOps execution v1 is not prepared")

// failureCode는 persisted JSON과 저장소 경계에서만 생기는 실패를 구분한다.
// lease 권한과 canonical CWD는 순수 domain 전이가 소유한다.
type failureCode string

// FailureInvalidState와 FailurePersistence는 Failure가 구분하는 실패 코드다.
// InvalidState는 persisted JSON이 계약에 맞지 않을 때, Persistence는 저장소 읽기·쓰기가 실패했을 때 쓴다.
const (
	FailureInvalidState failureCode = "invalid_state"
	FailurePersistence  failureCode = "persistence"
)

// Failure 타입은 persisted JSON과 저장소 경계에서 생긴 실패를 코드와 원인으로 감싼 오류다.
// Unwrap으로 원인을 노출하므로 errors.Is/As가 Cause까지 내려간다.
type Failure struct {
	Code  failureCode
	Cause error
}

func (e *Failure) Error() string { return fmt.Sprintf("%s: %v", e.Code, e.Cause) }
func (e *Failure) Unwrap() error { return e.Cause }

// Fail 함수는 code와 cause를 담은 *Failure를 error로 돌려준다.
func Fail(code failureCode, cause error) error {
	return &Failure{Code: code, Cause: cause}
}
