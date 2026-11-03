package trace

import (
	"strings"

	tracecontract "issueops/internal/contract/trace"
)

func ParseTraceparent(value string) (tracecontract.TraceContext, bool) {
	if len(value) != 55 || value[:3] != "00-" || value[35] != '-' || value[52] != '-' {
		return tracecontract.TraceContext{}, false
	}
	traceID, parentID, flags := value[3:35], value[36:52], value[53:]
	if !isLowerHex(traceID) || !isLowerHex(parentID) || !isLowerHex(flags) {
		return tracecontract.TraceContext{}, false
	}
	if strings.Trim(traceID, "0") == "" || strings.Trim(parentID, "0") == "" {
		return tracecontract.TraceContext{}, false
	}
	return tracecontract.TraceContext{TraceID: traceID, ParentID: parentID, Flags: flags}, true
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
