package trace

import (
	"strings"

	tracecontract "issueops/internal/contract/trace"
)

// ParseTraceparent 함수는 W3C traceparent 헤더 값(00-<trace-id>-<parent-id>-<flags>)을
// 파싱한다. 길이와 구분자가 맞고 세 필드가 소문자 16진수이며 trace-id와 parent-id가
// 모두 0이 아닐 때만 true를 돌려준다.
func ParseTraceparent(value string) (tracecontract.Context, bool) {
	if len(value) != 55 || value[:3] != "00-" || value[35] != '-' || value[52] != '-' {
		return tracecontract.Context{}, false
	}
	traceID, parentID, flags := value[3:35], value[36:52], value[53:]
	if !isLowerHex(traceID) || !isLowerHex(parentID) || !isLowerHex(flags) {
		return tracecontract.Context{}, false
	}
	if strings.Trim(traceID, "0") == "" || strings.Trim(parentID, "0") == "" {
		return tracecontract.Context{}, false
	}
	return tracecontract.Context{TraceID: traceID, ParentID: parentID, Flags: flags}, true
}

// isLowerHex는 field의 모든 문자가 소문자 16진수인지 검사한다.
func isLowerHex(field string) bool {
	for _, char := range field {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
			return false
		}
	}
	return true
}
