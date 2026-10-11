package failurecause

import (
	"sort"
	"strings"

	"issueops/internal/domain/policy"

	failurecausecontract "issueops/internal/contract/failurecause"
)

// causePriority는 증거에 여러 원인이 섞여 있을 때 먼저 채택할 원인 순서다.
var causePriority = []failurecausecontract.Cause{failurecausecontract.Transport, failurecausecontract.HarnessEnvironment, failurecausecontract.ContractInput, failurecausecontract.Model}

func Classify(failed bool, items []failurecausecontract.Evidence) failurecausecontract.Result {
	if !failed {
		return failurecausecontract.Result{Cause: failurecausecontract.None, Reason: "no_failed_steps", Evidence: []failurecausecontract.Evidence{}}
	}
	out := typedEvidence(items)
	sortEvidence(out)
	cause := primaryCause(out)
	reason := "no_typed_evidence"
	if cause != failurecausecontract.Unknown {
		reason = string(cause) + ":" + strings.Join(distinctCodes(out, cause), "+")
	}
	return failurecausecontract.Result{Cause: cause, Reason: reason, Evidence: out}
}

func isTypedCause(cause failurecausecontract.Cause) bool {
	return cause == failurecausecontract.Model || cause == failurecausecontract.HarnessEnvironment || cause == failurecausecontract.Transport || cause == failurecausecontract.ContractInput || cause == failurecausecontract.Unknown
}

// typedEvidence는 알려진 원인을 가진 증거만 code/source를 토큰화해 복사한다.
func typedEvidence(items []failurecausecontract.Evidence) []failurecausecontract.Evidence {
	out := make([]failurecausecontract.Evidence, 0, len(items))
	for _, e := range items {
		if isTypedCause(e.Cause) {
			e.Code = token(e.Code)
			e.Source = token(e.Source)
			out = append(out, e)
		}
	}
	return out
}

func sortEvidence(out []failurecausecontract.Evidence) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Cause != out[j].Cause {
			return out[i].Cause < out[j].Cause
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		return out[i].Source < out[j].Source
	})
}

// primaryCause는 causePriority 순서상 증거에 처음 등장하는 원인을 돌려주고, 없으면 Unknown이다.
func primaryCause(out []failurecausecontract.Evidence) failurecausecontract.Cause {
	for _, want := range causePriority {
		for _, e := range out {
			if e.Cause == want {
				return want
			}
		}
	}
	return failurecausecontract.Unknown
}

// distinctCodes는 정렬된 증거에서 cause에 해당하는 code를 연속 중복 없이 모은다.
func distinctCodes(out []failurecausecontract.Evidence, cause failurecausecontract.Cause) []string {
	codes := []string{}
	for _, e := range out {
		if e.Cause == cause && (len(codes) == 0 || codes[len(codes)-1] != e.Code) {
			codes = append(codes, e.Code)
		}
	}
	return codes
}

func token(s string) string {
	s = policy.RedactDiagnostic(s)
	if s == "<redacted>" || strings.Contains(s, "[REDACTED") {
		return "redacted"
	}
	var out strings.Builder
	separator := false
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == ':' || r == '-' {
			if separator {
				out.WriteByte('_')
			}
			out.WriteRune(r)
			separator = false
			continue
		}
		if out.Len() > 0 {
			separator = true
		}
	}
	value := strings.Trim(out.String(), "_")
	if value == "" {
		value = "unknown"
	}
	if len(value) > 96 {
		return value[:96]
	}
	return value
}
