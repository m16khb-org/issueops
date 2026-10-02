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
	for _, field := range []string{traceID, parentID, flags} {
		for _, char := range field {
			if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
				return tracecontract.TraceContext{}, false
			}
		}
	}
	if strings.Trim(traceID, "0") == "" || strings.Trim(parentID, "0") == "" {
		return tracecontract.TraceContext{}, false
	}
	return tracecontract.TraceContext{TraceID: traceID, ParentID: parentID, Flags: flags}, true
}
