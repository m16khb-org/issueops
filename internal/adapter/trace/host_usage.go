package trace

import (
	"bytes"
	"encoding/json"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"

	tracecontract "issueops/internal/contract/trace"
	tracedomain "issueops/internal/domain/trace"
)

const maxIdentityBytes = 256

type hostEvent = map[string]any

type hostCollector struct {
	observations []tracedomain.UsageObservation
	codes        map[string]bool
	intermediate bool
}

func isHostFormat(format string) bool {
	return format == tracecontract.InputFormatClaudeJSON || format == tracecontract.InputFormatCodexExec || format == tracecontract.InputFormatOmoJSON || format == tracecontract.InputFormatOmpJSON
}

func decodeHostUsage(body []byte, format string) tracedomain.Input {
	events, invalid, tooLong := parseHostEvents(body)
	input := tracedomain.Input{}
	if invalid {
		input.Warnings = append(input.Warnings, "invalid_jsonl_line")
	}
	if tooLong {
		input.Warnings = append(input.Warnings, "jsonl_line_too_long")
	}
	input.Incomplete = len(input.Warnings) > 0
	collector := &hostCollector{codes: map[string]bool{}}
	switch format {
	case tracecontract.InputFormatClaudeJSON:
		collector.claude(events)
	case tracecontract.InputFormatCodexExec:
		collector.codex(events)
	case tracecontract.InputFormatOmoJSON:
		collector.omo("omo", events)
	case tracecontract.InputFormatOmpJSON:
		// omp emits the same pi agent event stream and usage shape as Omo.
		collector.omo("omp", events)
	}
	warnings := append([]string{}, input.Warnings...)
	for code := range collector.codes {
		warnings = append(warnings, code)
	}
	input.Usage = tracedomain.ReduceUsage(collector.observations, warnings)
	return input
}

func parseHostEvents(body []byte) (events []hostEvent, invalid, tooLong bool) {
	for len(body) > 0 {
		line := body
		if index := bytes.IndexByte(body, '\n'); index >= 0 {
			line, body = body[:index], body[index+1:]
		} else {
			body = nil
		}
		line = bytes.TrimSuffix(line, []byte("\r"))
		if len(line) > tracecontract.HostLineMaxBytes {
			tooLong = true
			continue
		}
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		event, ok := decodeHostEvent(line)
		if !ok {
			invalid = true
			continue
		}
		events = append(events, event)
	}
	return events, invalid, tooLong
}

func decodeHostEvent(line []byte) (hostEvent, bool) {
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.UseNumber()
	var event hostEvent
	if err := decoder.Decode(&event); err != nil || event == nil {
		return nil, false
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, false
	}
	return event, true
}

func (c *hostCollector) warn(code string) { c.codes[code] = true }

func text(event hostEvent, key string) string {
	value, _ := event[key].(string)
	return strings.TrimSpace(value)
}

func object(event hostEvent, key string) hostEvent {
	value, _ := event[key].(map[string]any)
	return value
}

func hasUsageKey(event hostEvent) bool {
	_, usage := event["usage"]
	_, modelUsage := event["modelUsage"]
	return usage || modelUsage
}

func carriesUsage(event hostEvent) bool {
	return event["usage"] != nil || object(object(event, "message"), "usage") != nil
}

func (c *hostCollector) identity(value string) string {
	if len(value) > maxIdentityBytes {
		c.warn("usage_identity_too_long")
		return ""
	}
	return value
}

func label(value string) string {
	if len(value) > maxIdentityBytes {
		return ""
	}
	return value
}

func (c *hostCollector) metric(event hostEvent, key string) *int64 {
	raw, present := event[key]
	if !present || raw == nil {
		return nil
	}
	number, ok := raw.(json.Number)
	if !ok {
		c.warn("usage_invalid_metric")
		return nil
	}
	value, err := strconv.ParseInt(number.String(), 10, 64)
	if err != nil || value < 0 {
		c.warn("usage_invalid_metric")
		return nil
	}
	return &value
}

func (c *hostCollector) cost(event hostEvent, key string) *float64 {
	raw, present := event[key]
	if !present || raw == nil {
		return nil
	}
	number, ok := raw.(json.Number)
	if !ok {
		c.warn("usage_invalid_metric")
		return nil
	}
	value, err := strconv.ParseFloat(number.String(), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		c.warn("usage_invalid_metric")
		return nil
	}
	return &value
}

func (c *hostCollector) add(observation tracedomain.UsageObservation) {
	if observation.Finality != tracecontract.UsageFinalityFinal {
		clearZeros(&observation)
	}
	observation.CostBasis = tracecontract.UsageCostBasisUnknown
	if observation.CostUSD != nil {
		observation.CostBasis = tracecontract.UsageCostBasisHostEstimate
	}
	c.observations = append(c.observations, observation)
}

func clearZeros(observation *tracedomain.UsageObservation) {
	for _, metric := range []**int64{&observation.Input, &observation.Output, &observation.CacheRead, &observation.CacheWrite} {
		if *metric != nil && **metric == 0 {
			*metric = nil
		}
	}
	if observation.CostUSD != nil && *observation.CostUSD == 0 {
		observation.CostUSD = nil
	}
}

func finalityOf(final bool) string {
	if final {
		return tracecontract.UsageFinalityFinal
	}
	return tracecontract.UsageFinalityPartial
}

var claudeKnownEvents = map[string]bool{
	"assistant": true, "user": true, "stream_event": true, "rate_limit_event": true, "keep_alive": true,
	"tool_progress": true, "auth_status": true, "control_request": true, "control_response": true, "control_cancel_request": true,
}

func (c *hostCollector) claude(events []hostEvent) {
	versions := map[string]bool{}
	for _, event := range events {
		switch kind := text(event, "type"); kind {
		case "system":
			if version := text(event, "claude_code_version"); version != "" && len(version) <= maxIdentityBytes {
				versions[version] = true
			}
		case "result":
			c.claudeResult(event)
		default:
			if claudeKnownEvents[kind] {
				c.intermediate = c.intermediate || carriesUsage(event)
			} else if hasUsageKey(event) {
				c.warn("usage_unknown_event")
			}
		}
	}
	if len(c.observations) == 0 && c.intermediate {
		c.warn("usage_final_missing")
	}
	if len(versions) == 1 {
		for version := range versions {
			for index := range c.observations {
				c.observations[index].Version = version
			}
		}
	}
}

func (c *hostCollector) claudeResult(event hostEvent) {
	session := c.identity(text(event, "session_id"))
	message := c.identity(text(event, "uuid"))
	final := text(event, "subtype") == "success"
	if raw, present := event["is_error"]; present {
		isError, valid := raw.(bool)
		if !valid {
			c.warn("usage_invalid_status")
		}
		final = final && valid && !isError
	}
	models := object(event, "modelUsage")
	if len(models) == 0 {
		c.warn("usage_missing")
		return
	}
	names := make([]string, 0, len(models))
	for name := range models {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := object(models, name)
		if entry == nil {
			c.warn("usage_missing")
			continue
		}
		c.add(tracedomain.UsageObservation{
			Host: "claude", Provider: label(text(entry, "provider")), Model: label(name),
			SessionID: session, MessageID: message,
			Finality: finalityOf(final), Temporality: tracecontract.UsageTemporalityCumulative,
			Input: c.metric(entry, "inputTokens"), Output: c.metric(entry, "outputTokens"),
			CacheRead: c.metric(entry, "cacheReadInputTokens"), CacheWrite: c.metric(entry, "cacheCreationInputTokens"),
			CostUSD: c.cost(entry, "costUSD"),
		})
	}
}

func (c *hostCollector) codex(events []hostEvent) {
	thread := ""
	for _, event := range events {
		switch kind := text(event, "type"); kind {
		case "thread.started":
			thread = c.identity(text(event, "thread_id"))
		case "turn.completed":
			usage := object(event, "usage")
			if usage == nil {
				c.warn("usage_missing")
				continue
			}
			c.add(tracedomain.UsageObservation{
				Host: "codex", SessionID: thread, TurnID: c.identity(text(event, "turn_id")),
				Finality: tracecontract.UsageFinalityFinal, Temporality: tracecontract.UsageTemporalityDelta,
				Input: c.metric(usage, "input_tokens"), Output: c.metric(usage, "output_tokens"),
				CacheRead: c.metric(usage, "cached_input_tokens"),
			})
		case "turn.failed":
			c.warn("usage_turn_failed")
		case "turn.started", "item.started", "item.updated", "item.completed", "error":
		default:
			if hasUsageKey(event) {
				c.warn("usage_unknown_event")
			}
		}
	}
}

func (c *hostCollector) omo(host string, events []hostEvent) {
	session := ""
	for _, event := range events {
		switch kind := text(event, "type"); kind {
		case "session":
			session = c.identity(text(event, "id"))
		case "message_end":
			c.omoMessageEnd(host, session, event)
		case "message_update":
			c.intermediate = c.intermediate || event["usage"] != nil
		case "turn_end":
			c.intermediate = c.intermediate || carriesUsage(event)
		case "agent_start", "agent_end", "turn_start", "message_start", "tool_execution_start", "tool_execution_update", "tool_execution_end":
		default:
			if hasUsageKey(event) {
				c.warn("usage_unknown_event")
			}
		}
	}
	if len(c.observations) == 0 && c.intermediate {
		c.warn("usage_final_missing")
	}
}

func (c *hostCollector) omoMessageEnd(host, session string, event hostEvent) {
	message := object(event, "message")
	if text(message, "role") != "assistant" {
		return
	}
	usage := object(message, "usage")
	if usage == nil {
		c.warn("usage_missing")
		return
	}
	id := text(message, "responseId")
	if id == "" {
		id = text(message, "id")
	}
	finality := tracecontract.UsageFinalityUnknown
	switch text(message, "stopReason") {
	case "stop", "length", "toolUse":
		finality = tracecontract.UsageFinalityFinal
	case "error", "aborted":
		finality = tracecontract.UsageFinalityPartial
	}
	c.add(tracedomain.UsageObservation{
		Host: host, Provider: label(text(message, "provider")), Model: label(text(message, "model")),
		SessionID: session, MessageID: c.identity(id),
		Finality: finality, Temporality: tracecontract.UsageTemporalityDelta,
		Input: c.metric(usage, "input"), Output: c.metric(usage, "output"),
		CacheRead: c.metric(usage, "cacheRead"), CacheWrite: c.metric(usage, "cacheWrite"),
		CostUSD: c.omoCost(usage),
	})
}

func (c *hostCollector) omoCost(usage hostEvent) *float64 {
	total := c.cost(object(usage, "cost"), "total")
	if total != nil && *total == 0 {
		return nil
	}
	return total
}
