package trace

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"

	tracecontract "issueops/internal/contract/trace"
	"issueops/internal/domain/policy"
)

// UsageObservation 타입은 호스트가 보고한 토큰·비용 사용량 관측 하나다. Temporality가
// cumulative이면 세션 누적값이고 아니면 turn/message 단위 증분값이다. 포인터 필드가
// nil이면 호스트가 그 값을 보고하지 않았다는 뜻이다.
type UsageObservation struct {
	Host, Version, Provider, Model string
	SessionID, TurnID, MessageID   string
	Finality, Temporality          string
	Input, Output                  *int64
	CacheRead, CacheWrite          *int64
	CostUSD                        *float64
	CostBasis                      string
}

type usageEntry struct {
	observation UsageObservation
	epoch       int
	identified  bool
}

type cumulativeKey struct{ host, session, provider, model string }

type deltaKey struct{ host, session, turn, message string }

func usageDigest(kind, id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(kind + "\x00" + id))
	return hex.EncodeToString(sum[:])
}

// ReduceUsage 함수는 관측들을 UsageReport로 합친다. 누적값은 세션별로 최신 값만
// 남기고(값이 줄면 reset으로 보고 새 구간을 시작), turn/message가 같은 증분 중복은
// 하나만 남기며, 식별자가 없는 관측은 그대로 둔다. 경고 코드는 정렬해 담는다.
// 커버리지는 관측이 없거나 식별되지 않은 표본이 있으면 unknown, 최종값이 아니거나
// 중복 외의 경고가 있으면 partial, 그 밖에는 complete다.
func ReduceUsage(observations []UsageObservation, warnings []string) *tracecontract.UsageReport {
	entries := []usageEntry{}
	current := map[cumulativeKey]int{}
	epochs := map[cumulativeKey]int{}
	seen := map[deltaKey]int{}
	codes := map[string]bool{}
	for _, code := range warnings {
		codes[code] = true
	}
	for _, observation := range observations {
		if observation.Temporality == tracecontract.UsageTemporalityCumulative {
			if observation.SessionID == "" {
				entries = append(entries, usageEntry{observation: observation})
				continue
			}
			key := cumulativeKey{observation.Host, observation.SessionID, observation.Provider, observation.Model}
			index, ok := current[key]
			if !ok {
				current[key] = len(entries)
				entries = append(entries, usageEntry{observation: observation, identified: true})
				continue
			}
			if usageDecreased(entries[index].observation, observation) {
				codes["usage_cumulative_reset"] = true
				epochs[key]++
				current[key] = len(entries)
				entries = append(entries, usageEntry{observation: observation, epoch: epochs[key], identified: true})
				continue
			}
			entries[index].observation = observation
			continue
		}
		if observation.TurnID == "" && observation.MessageID == "" {
			entries = append(entries, usageEntry{observation: observation})
			continue
		}
		key := deltaKey{observation.Host, observation.SessionID, observation.TurnID, observation.MessageID}
		index, ok := seen[key]
		if !ok {
			seen[key] = len(entries)
			entries = append(entries, usageEntry{observation: observation, identified: true})
			continue
		}
		if sameUsage(entries[index].observation, observation) {
			codes["usage_duplicate_event"] = true
		} else {
			codes["usage_conflicting_duplicate"] = true
		}
	}
	report := &tracecontract.UsageReport{Samples: make([]tracecontract.UsageSample, 0, len(entries)), Coverage: usageCoverage(entries, codes)}
	for _, entry := range entries {
		report.Samples = append(report.Samples, usageSample(entry))
	}
	report.Warnings = sortedCodes(codes)
	return report
}

// NoteUsageLoss 함수는 report에 손실 경고 code를 추가하고(기존 경고는 유지, 정렬)
// 커버리지가 complete이면 partial로 낮춘다. report를 제자리에서 바꾼다.
func NoteUsageLoss(report *tracecontract.UsageReport, code string) {
	codes := map[string]bool{code: true}
	for _, existing := range report.Warnings {
		codes[existing] = true
	}
	report.Warnings = sortedCodes(codes)
	if report.Coverage == tracecontract.UsageCoverageComplete {
		report.Coverage = tracecontract.UsageCoveragePartial
	}
}

func usageCoverage(entries []usageEntry, codes map[string]bool) string {
	if len(entries) == 0 {
		return tracecontract.UsageCoverageUnknown
	}
	coverage := tracecontract.UsageCoverageComplete
	for _, entry := range entries {
		if !entry.identified {
			return tracecontract.UsageCoverageUnknown
		}
		if entry.observation.Finality != tracecontract.UsageFinalityFinal {
			coverage = tracecontract.UsageCoveragePartial
		}
	}
	for code := range codes {
		if code != "usage_duplicate_event" {
			coverage = tracecontract.UsageCoveragePartial
		}
	}
	return coverage
}

func usageSample(entry usageEntry) tracecontract.UsageSample {
	o := entry.observation
	scope := ""
	if o.SessionID != "" {
		scope = usageDigest("scope", o.Host+"\x00"+o.SessionID)
	}
	return tracecontract.UsageSample{
		Host: o.Host, Version: policy.RedactDiagnostic(o.Version), ScopeID: scope,
		SessionID: usageDigest("session", o.SessionID), TurnID: usageDigest("turn", o.TurnID), MessageID: usageDigest("message", o.MessageID),
		Provider: policy.RedactDiagnostic(o.Provider), Model: policy.RedactDiagnostic(o.Model),
		Finality:    valueOrUnknown(o.Finality),
		Temporality: o.Temporality,
		Epoch:       strconv.Itoa(entry.epoch),
		CostBasis:   valueOrUnknown(o.CostBasis),
		InputTokens: o.Input, OutputTokens: o.Output, CacheReadTokens: o.CacheRead, CacheWriteTokens: o.CacheWrite,
		CostUSD: o.CostUSD,
	}
}

func valueOrUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

func sortedCodes(codes map[string]bool) []string {
	out := make([]string, 0, len(codes))
	for code := range codes {
		out = append(out, code)
	}
	sort.Strings(out)
	return out
}

func usageDecreased(previous, next UsageObservation) bool {
	return int64Decreased(previous.Input, next.Input) || int64Decreased(previous.Output, next.Output) ||
		int64Decreased(previous.CacheRead, next.CacheRead) || int64Decreased(previous.CacheWrite, next.CacheWrite) ||
		(previous.CostUSD != nil && next.CostUSD != nil && *next.CostUSD < *previous.CostUSD)
}

func int64Decreased(previous, next *int64) bool {
	return previous != nil && next != nil && *next < *previous
}

func sameUsage(a, b UsageObservation) bool {
	return sameInt64(a.Input, b.Input) && sameInt64(a.Output, b.Output) && sameInt64(a.CacheRead, b.CacheRead) && sameInt64(a.CacheWrite, b.CacheWrite) &&
		((a.CostUSD == nil && b.CostUSD == nil) || (a.CostUSD != nil && b.CostUSD != nil && *a.CostUSD == *b.CostUSD))
}

func sameInt64(a, b *int64) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
