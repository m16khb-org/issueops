# IssueOps trace, review metrics, token cost, and visibility

Retrieved and inspected: **2026-10-02**. Scope: current checkout and public official sources; no production changes, installations, runtime-state inspection, tests, or benchmarks. Model: `gpt-6.1-sol` (`PI_MODEL`). Performance improvements below are hypotheses, not measured results.

## Findings

### 1. IssueOps trace analysis is not distributed tracing

**High certainty, direct code evidence.** `internal/adapter/trace/input.go:13-25` loads a complete file, stdin, or state record. `internal/adapter/trace/decode.go:40-80` projects failure summaries, guard rules, progress events, and documentation-upkeep events. The response exposes findings and warnings, not W3C trace/span IDs (`internal/contract/trace/types.go:9-32`). Repository-wide searches across Go, JS, TS, and shell found no `TRACEPARENT`, `traceparent`, `trace_id`, or `span_id` implementation.

**Applicability:** preserve this useful diagnostic surface rather than relabeling it an OTel collector. **Counterevidence:** neighboring tests already cover state-key ingestion and evidence redaction (`internal/adapter/trace/analyze_test.go:191-223`); visibility is not absent. Audit identifiers are also distinct from distributed trace context.

### 2. Hook-free correlation is an optional integration candidate

**High certainty as an official single-source claim, not runtime corroboration.** [Claude Code monitoring](https://code.claude.com/docs/en/monitoring-usage#traces-beta) states that active beta tracing passes W3C `TRACEPARENT` to Bash/PowerShell subprocesses. Tracing is disabled by default and requires telemetry, beta tracing, and an exporter selection. With a custom `ANTHROPIC_BASE_URL`, propagation requires `CLAUDE_CODE_PROPAGATE_TRACEPARENT=1`. The same page says subprocesses do **not** inherit `OTEL_*` exporter settings.

**Applicability:** consider opt-in correlation-only metadata at the CLI boundary, without hooks, exporters, or host-specific core behavior. **Counterevidence:** propagation alone neither exports IssueOps spans nor covers all hosts. The [W3C Trace Context Level 1 Recommendation, released 2021-11-23](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/#traceparent-header), independently defines trace/parent IDs and invalid-value handling, but does not corroborate Claude's implementation. Claude's living page gives no introduction version or release date for subprocess propagation; no future release is inferred.

### 3. Existing review metrics need denominator-aware interpretation

**High certainty, implementation plus neighboring tests.** Completed-phase durations are timestamp differences; unfinished phases are omitted (`internal/domain/issueops/review_metrics.go:70-91`; `internal/domain/issueops/review_metrics_test.go:56-63`). Mean rounds divides by reviewed cycles, while revise/stop ratios divide by all recorded verdicts (`internal/domain/issueops/review_metrics.go:95-120`). Repository aggregation excludes unreadable records and exposes `read_errors` and warnings (`internal/application/issueopsreview/metrics.go:28-59`; `internal/application/issueopsreview/metrics_test.go:12-51`).

**Applicability:** use these existing fields for review-throughput baselines before adding metrics. Report population, exclusions, and unfinished phases alongside comparisons. **Counterevidence:** phase residence includes human/idle time and is neither CPU time nor token spend; a lower revise ratio alone does not establish better quality or speed. Code and tests are mutually supporting repository evidence, not independent external corroboration.

### 4. Historical token accounting is not evidence of a current writer

**High certainty within the searched checkout.** The dated decision `.issueops/adr/decisions/2026-07-02-external-llm-usage-observation.md:7-14` describes a best-effort writer and names `internal/core/externalllm/*` and `internal/core/external_llm_usage*`. Those paths are absent from the tracked file inventory. Searches for `RunExternalLLMPrint`, provider token fields, and `external-llm-usage` across executable-source extensions found only the latter's retention fixtures (`internal/adapter/outbound/state/state_test.go:340-360`).

**Applicability:** reconcile documented ownership before claiming present per-call accounting or implementing cost budgets. **Counterevidence:** generic storage can retain historical usage keys, and this search does not inspect installed older binaries or host-owned usage. The ADR date is a decision date, not a verified release date.

### 5. Host cost telemetry is approximate; local parsing has visibility limits

**High certainty about documented/code behavior; impact unmeasured.** [Monitoring cost monitoring](https://code.claude.com/docs/en/monitoring-usage#cost-monitoring) explicitly distinguishes approximate cost metrics from official provider billing. It documents a streaming-usage overcount fixed in **v2.1.214**; release date is not supplied. Token counters distinguish input/output/cache-read/cache-creation. Locally, JSONL decoding skips malformed lines and returns without checking `scanner.Err()` (`internal/adapter/trace/decode.go:26-38`).

**Applicability:** label host-derived cost estimates and preserve model/version/cache dimensions. Investigate explicit parser-loss reporting before performance optimization. **Counterevidence:** malformed-line tolerance enables recovery; no large-input failure, overhead, or savings were measured.

## Retrieval and verification

Claude webfetch output hit its 50 KB cap. Complete Markdown was independently fetched from `https://code.claude.com/docs/en/monitoring-usage.md`: HTTP 200, **139,553 bytes**, SHA-256 `aa12cfdf0b859a3b7a1ec2f598fcade2886cbcc3fce179196207114da8c73e13`. Bounded extracts retained propagation, cost approximation, counters, and version evidence in the session. Both Claude URLs were reopened with webfetch. W3C webfetch calls returned no content for the dated and latest URLs; direct retrieval recovered the dated specification, HTTP 200, 100,612 bytes. No source remained inaccessible; no shell command failed. Cited implementation/test ranges were reread. Tests/build/benchmarks were intentionally excluded.

## EXPAND

- **E03 / candidate:** validate optional W3C correlation metadata, malformed-context rejection, and no-context behavior; no mandatory exporter.
- **E05 / actionable:** reconcile the historical usage-writer ADR with current code ownership.
- **E06 / actionable:** investigate oversized JSONL and scanner-error visibility with focused regression fixtures.
- **E07 / measurement prerequisite:** compare review populations and host-version-tagged token estimates before any optimization or budget rule.
