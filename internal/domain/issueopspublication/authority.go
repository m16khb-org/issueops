package issueopspublication

import (
	"fmt"
	"strings"
)

// ValidateOperationID 함수는 operationID가 소문자 16진수 32자인지 검사하고 아니면 오류를 반환한다.
func ValidateOperationID(operationID string) error {
	if len(operationID) == 32 {
		valid := true
		for _, char := range []byte(operationID) {
			if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
				valid = false
				break
			}
		}
		if valid {
			return nil
		}
	}
	return fmt.Errorf("remote operation ID must be exactly 32 lowercase hexadecimal characters")
}

// BeginAuthorityFacts 타입은 원격 생성 의도를 기록하기 직전에 확인하는 사실이다.
// 준비 여부, 이미 대기 중인 의도나 산출물의 존재, 현재와 기대 lease 세대를 담는다.
type BeginAuthorityFacts struct {
	Prepared           bool
	Pending            bool
	Artifact           bool
	CurrentGeneration  uint64
	ExpectedGeneration uint64
}

// ValidateBeginAuthority 함수는 의도를 기록하기 전에 권한이 그대로인지 검사한다.
// 준비되지 않았거나 대기 의도 또는 산출물이 이미 있으면, 그리고 기대 세대가 0이거나 현재 세대와 다르면 오류를 반환한다.
func ValidateBeginAuthority(facts BeginAuthorityFacts) error {
	if !facts.Prepared || facts.Pending || facts.Artifact {
		return fmt.Errorf("remote create authority changed before intent CAS")
	}
	if facts.ExpectedGeneration == 0 || facts.CurrentGeneration != facts.ExpectedGeneration {
		return fmt.Errorf("stale lease generation before remote intent CAS")
	}
	return nil
}

// ReceiptAuthorityFacts 타입은 원격 영수증을 기록하기 직전에 확인하는 사실이다.
// 대기 중인 의도의 작업 ID, 현재 lease 세대와 보유자, 기대값, 그리고 CWD 일치 여부를 담는다.
type ReceiptAuthorityFacts struct {
	Prepared            bool
	Pending             bool
	PendingOperationID  string
	ExpectedOperationID string
	Generation          uint64
	ExpectedGeneration  uint64
	LeaseStatus         string
	HolderPresent       bool
	HolderHost          string
	ExpectedHost        string
	HolderSessionID     string
	ExpectedSessionID   string
	HolderAgentID       string
	ExpectedAgentID     string
	CWDMatches          bool
}

// ValidateReceiptAuthority 함수는 영수증을 기록하기 전에 대기 의도가 같은 작업의 것인지 검사한다.
// enforceOriginalGeneration이 true이면 lease가 원래 세대에서 활성이고 보유자와 CWD가 그대로인지도 검사한다.
// 조건을 어기면 오류를 반환한다.
func ValidateReceiptAuthority(facts ReceiptAuthorityFacts, enforceOriginalGeneration bool) error {
	if !facts.Prepared || !facts.Pending || facts.PendingOperationID != facts.ExpectedOperationID {
		return fmt.Errorf("external intent changed before remote receipt CAS")
	}
	if enforceOriginalGeneration && receiptGenerationStale(facts) {
		return fmt.Errorf("remote receipt belongs to a stale execution generation; execution reconcile is required")
	}
	return nil
}

// receiptGenerationStale은 영수증 소유 실행 세대가 기대한 세대·보유자와 다른지 판단한다.
func receiptGenerationStale(facts ReceiptAuthorityFacts) bool {
	return facts.ExpectedGeneration == 0 || facts.Generation != facts.ExpectedGeneration ||
		facts.LeaseStatus != "active" || !facts.HolderPresent ||
		!strings.EqualFold(facts.HolderHost, facts.ExpectedHost) || facts.HolderSessionID != facts.ExpectedSessionID ||
		facts.HolderAgentID != facts.ExpectedAgentID || !facts.CWDMatches
}
