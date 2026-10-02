package trace

import (
	"bufio"
	"encoding/json"
	tracecontract "issueops/internal/contract/trace"
	tracedomain "issueops/internal/domain/trace"
	"strings"
)

func (Source) Decode(body []byte, format string) tracedomain.Input {
	switch {
	case format == "" || format == tracecontract.InputFormatIssueOps:
		return decodeIssueOps(body)
	case isHostFormat(format):
		return decodeHostUsage(body, format)
	default:
		return tracedomain.Input{Incomplete: true, Warnings: []string{"unsupported_input_format"}}
	}
}

func decodeIssueOps(body []byte) tracedomain.Input {
	text := strings.TrimSpace(string(body))
	result := tracedomain.Input{}
	if strings.HasPrefix(text, "{") {
		var doc map[string]any
		if err := json.Unmarshal([]byte(text), &doc); err == nil {
			observed := observeDocument(doc)
			result.Document = &observed
			return result
		}
		if !strings.Contains(text, "\n") {
			result.Incomplete = true
			result.Warnings = []string{"invalid_json:invalid_jsonl_line"}
			return result
		}
	}
	scanner := bufio.NewScanner(strings.NewReader(text))
	invalidLine := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var doc map[string]any
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			invalidLine = true
			continue
		}
		result.Lines = append(result.Lines, observeDocument(doc))
	}
	if invalidLine {
		result.Incomplete = true
		result.Warnings = append(result.Warnings, "invalid_jsonl_line")
	}
	if scanner.Err() != nil {
		result.Incomplete = true
		result.Warnings = append(result.Warnings, "jsonl_scan_error")
	}
	return result
}
func observeDocument(doc map[string]any) tracedomain.Document {
	summary := nestedMap(doc, "summary")
	hasSummary := summary != nil
	if summary == nil {
		summary = doc
	}
	observed := tracedomain.Document{HasSummary: hasSummary, HasGuard: nestedMap(doc, "guard") != nil, TopFailureClass: stringField(doc, "failure_class"), TopFailedSteps: intField(doc, "failed_steps"), Event: stringField(doc, "event"), Step: stringField(doc, "step"), OK: boolField(doc, "ok")}
	observed.Summary = tracedomain.Summary{FailedSteps: intField(summary, "failed_steps"), FailedStep: stringField(summary, "failed_step"), FailureClass: stringField(summary, "failure_class"), RerunCommands: stringSliceField(summary, "rerun_commands")}
	if clusters, ok := summary["failure_clusters"].([]any); ok {
		for _, raw := range clusters {
			if cluster, ok := raw.(map[string]any); ok {
				observed.Summary.Clusters = append(observed.Summary.Clusters, tracedomain.Cluster{Step: stringField(cluster, "step"), Count: intField(cluster, "count")})
			}
		}
	}
	if items, ok := summary["failure_cause_evidence"].([]any); ok {
		for _, raw := range items {
			if entry, ok := raw.(map[string]any); ok {
				observed.Summary.Evidence = append(observed.Summary.Evidence, tracedomain.Evidence{Cause: stringField(entry, "cause"), Code: stringField(entry, "code"), Source: stringField(entry, "source")})
			}
		}
	}
	guard := nestedMap(doc, "guard")
	if guard == nil {
		guard = doc
	}
	if findings, ok := guard["findings"].([]any); ok {
		for _, raw := range findings {
			if finding, ok := raw.(map[string]any); ok {
				observed.GuardRules = append(observed.GuardRules, stringField(finding, "rule"))
			}
		}
	}
	event := nestedMap(doc, "event")
	if event == nil {
		event = doc
	}
	observed.Upkeep = tracedomain.Upkeep{Kind: stringField(event, "kind"), Summary: stringField(event, "summary"), TargetDocs: stringSliceField(event, "target_docs")}
	return observed
}
