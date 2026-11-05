package issueopspublication

import (
	"fmt"
	"reflect"
	"strings"

	contract "issueops/internal/contract/issueopspublication"
)

type intentCheckpoint string

// 의도 체크포인트 이름: 상태 전이 검사가 오류 메시지에 쓰는 단계 이름이다.
const (
	CheckpointRetry      intentCheckpoint = "retry CAS"
	CheckpointFailure    intentCheckpoint = "failure receipt"
	CheckpointReceipt    intentCheckpoint = "remote receipt CAS"
	CheckpointNotInvoked intentCheckpoint = "terminal pre-invocation receipt"
)

// PendingIntentFacts 타입은 대기 중인 의도가 아직 같은 작업의 것인지 확인하는 사실이다.
// 준비 여부, 대기 여부, 저장된 작업 ID와 기대 작업 ID를 담는다.
type PendingIntentFacts struct {
	Prepared            bool
	Pending             bool
	OperationID         string
	ExpectedOperationID string
}

// ValidatePendingIntent 함수는 checkpoint 시점에 의도가 준비되고 대기 중이며 기대한 작업 ID와 같은지 검사한다.
// 아니면 의도가 바뀌었다는 오류를 반환한다.
func ValidatePendingIntent(facts PendingIntentFacts, checkpoint intentCheckpoint) error {
	if !facts.Prepared || !facts.Pending || facts.OperationID != facts.ExpectedOperationID {
		return fmt.Errorf("external intent changed before %s", checkpoint)
	}
	return nil
}

// ValidatePayloadUnchanged 함수는 저장된 의도 페이로드가 expected와 같은지 검사하고, 다르면 checkpoint 이름이 담긴 오류를 반환한다.
func ValidatePayloadUnchanged(stored, expected contract.IntentPayload, checkpoint intentCheckpoint) error {
	if !reflect.DeepEqual(stored, expected) {
		return fmt.Errorf("external intent payload changed before %s", checkpoint)
	}
	return nil
}

// RetryPayload 함수는 호출 상태를 unknown으로 되돌리고 재시도 횟수를 1 늘린 페이로드 복사본을 반환한다.
func RetryPayload(payload contract.IntentPayload) contract.IntentPayload {
	payload.InvocationState = string(contract.InvocationUnknown)
	payload.RetryCount++
	return payload
}

// FailurePayload 함수는 호출 상태와 재시도 횟수를 바꾼 페이로드 복사본을 반환한다.
// knownURL이 공백이 아니면 앞뒤 공백을 지워 KnownURL로 기록하고, 비어 있으면 기존 값을 유지한다.
func FailurePayload(payload contract.IntentPayload, invocation string, retryCount int, knownURL string) contract.IntentPayload {
	payload.InvocationState, payload.RetryCount = invocation, retryCount
	if strings.TrimSpace(knownURL) != "" {
		payload.KnownURL = strings.TrimSpace(knownURL)
	}
	return payload
}

// ValidatePublicationPending 함수는 게시 의도가 준비되고 대기 중이며 종류가 원격 PR 생성인지 검사하고, 아니면 오류를 반환한다.
func ValidatePublicationPending(prepared, pending bool, kind string) error {
	if !prepared || !pending || kind != contract.RemoteIntentKind {
		return fmt.Errorf("remote publication intent is not pending")
	}
	return nil
}
