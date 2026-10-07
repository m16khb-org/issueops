package inspect

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	inspectcontract "issueops/internal/contract/inspect"
)

const (
	maxReceiptBytes     = 1 << 20
	maxArtifactBytes    = 64 << 20
	hostVersionTimeout  = 10 * time.Second
	hostVersionMaxBytes = 4096
)

var (
	versionPattern  = regexp.MustCompile(`\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.+-]*)?`)
	schemePattern   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
	hostBinaryNames = map[string]string{
		inspectcontract.HostCodex:  "codex",
		inspectcontract.HostClaude: "claude",
		inspectcontract.HostOmo:    "omo",
	}
)

func (observer Observer) applyReceipts(hosts []inspectcontract.HostIntegration, entries []hostEntry, receiptPath, observedAt string) {
	if receiptPath == "" {
		return
	}
	resolved, code := observer.resolveReceiptPath(receiptPath)
	var receipts map[string]inspectcontract.HostIntegration
	if code == "" {
		receipts, code = loadReceipts(resolved)
	}
	if code != "" {
		for i := range hosts {
			unknown := observation(inspectcontract.ObservationUnknown, receiptPath, observedAt, code)
			hosts[i].Discovered, hosts[i].Connected, hosts[i].Protocol = unknown, unknown, unknown
		}
		return
	}
	for i := range hosts {
		receipt, ok := receipts[hosts[i].Host]
		if !ok {
			missing := notChecked()
			missing.Source = resolved
			missing.Reason = "no_receipt_for_host"
			hosts[i].Discovered, hosts[i].Connected, hosts[i].Protocol = missing, missing, missing
			continue
		}
		vetter := &receiptVetter{
			observer:   observer,
			host:       hosts[i].Host,
			transport:  hosts[i].Transport,
			configPath: hosts[i].ConfigPath,
			configSHA:  entries[i].sha256,
			receipt:    receipt,
			receiptDir: filepath.Dir(resolved),
		}
		hosts[i].Discovered = vetter.vet(receipt.Discovered, "tools/list")
		hosts[i].Connected = vetter.vet(receipt.Connected, "docs_index")
		hosts[i].Protocol = vetter.vet(receipt.Protocol, "")
	}
}

func (observer Observer) resolveReceiptPath(path string) (string, string) {
	if observer.ReceiptRoot == "" {
		return path, ""
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(observer.ReceiptRoot, path)
	}
	root, err := filepath.EvalSymlinks(observer.ReceiptRoot)
	if err != nil {
		return path, "receipt_unreadable"
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, "receipt_unreadable"
	}
	if rel, err := filepath.Rel(root, resolved); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path, "receipt_outside_workspace"
	}
	return resolved, ""
}

func loadReceipts(path string) (map[string]inspectcontract.HostIntegration, string) {
	file, err := os.Open(path)
	if err != nil {
		return nil, "receipt_unreadable"
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxReceiptBytes {
		return nil, "receipt_unreadable"
	}
	body, err := io.ReadAll(io.LimitReader(file, maxReceiptBytes+1))
	if err != nil {
		return nil, "receipt_unreadable"
	}
	var parsed inspectcontract.HostReceipts
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil || decoder.More() {
		return nil, "receipt_malformed"
	}
	if parsed.SchemaVersion != inspectcontract.HostReceiptsSchemaVersion {
		return nil, "receipt_schema_unsupported"
	}
	receipts := make(map[string]inspectcontract.HostIntegration, len(parsed.Hosts))
	for _, host := range parsed.Hosts {
		if _, duplicate := receipts[host.Host]; duplicate {
			return nil, "receipt_duplicate_host"
		}
		receipts[host.Host] = host
	}
	return receipts, ""
}

type receiptVetter struct {
	observer   Observer
	host       string
	transport  string
	configPath string
	configSHA  string
	receipt    inspectcontract.HostIntegration
	receiptDir string

	versionProbed  bool
	currentVersion string
}

// vet은 receipt의 한 관측을 현재 상태와 대조한다. verified는 실제 artifact,
// 현재 config·host version·transport와의 일치, 해당 관측에 필요한 증거가
// 모두 있을 때만 유지한다. 하나라도 어긋나면 unknown으로 내린다.
func (vetter *receiptVetter) vet(observed inspectcontract.Observation, requiredFeature string) inspectcontract.Observation {
	result := observed
	result.Features = append([]string{}, observed.Features...)
	switch observed.Status {
	case inspectcontract.ObservationNotChecked:
		return result
	case inspectcontract.ObservationVerified, inspectcontract.ObservationFailed, inspectcontract.ObservationUnknown:
	default:
		return demote(result, "receipt_status_invalid")
	}
	if reason := vetter.staleReason(observed); reason != "" {
		return demote(result, reason)
	}
	source, reason := vetter.artifact(observed.Source)
	if reason != "" {
		return demote(result, reason)
	}
	result.Source = source
	if _, err := time.Parse(time.RFC3339, observed.ObservedAt); err != nil {
		return demote(result, "receipt_observed_at_invalid")
	}
	if observed.Status != inspectcontract.ObservationVerified {
		return result
	}
	if requiredFeature != "" && !slices.Contains(observed.Features, requiredFeature) {
		return demote(result, "receipt_missing_"+strings.ReplaceAll(requiredFeature, "/", "_"))
	}
	if requiredFeature == "" && observed.RequestedRevision == "" && observed.NegotiatedRevision == "" {
		return demote(result, "receipt_missing_revision")
	}
	if reason := artifactEvidence(source, requiredFeature, observed); reason != "" {
		return demote(result, reason)
	}
	return result
}

// artifactEvidence는 verified 관측의 근거를 artifact의 JSONL 이벤트 의미로 확인한다. 발견은 host
// catalog가 docs_index를 나열한 이벤트가, 연결은 오류 없이 끝난 docs_index 결과가, protocol은
// 응답 필드에 실린 협상 revision이 있어야 한다. 이름이나 revision이 문장에 언급된 것만으로는
// 근거가 되지 않고, 인식하지 못하는 형식은 근거 없음으로 남는다.
func artifactEvidence(path, requiredFeature string, observed inspectcontract.Observation) string {
	facts, ok := readArtifactFacts(path)
	if !ok {
		return "receipt_source_not_artifact"
	}
	switch requiredFeature {
	case "":
		revision := observed.NegotiatedRevision
		if revision == "" {
			revision = observed.RequestedRevision
		}
		if !facts.revisions[revision] {
			return "receipt_artifact_missing_revision"
		}
	case "docs_index":
		if facts.docsFailed {
			return "receipt_artifact_tool_failed"
		}
		if !facts.docsSucceeded {
			return "receipt_artifact_missing_evidence"
		}
	default:
		if !facts.catalogListsDocs {
			return "receipt_artifact_missing_evidence"
		}
	}
	return ""
}

type artifactFacts struct {
	catalogListsDocs bool
	docsSucceeded    bool
	docsFailed       bool
	revisions        map[string]bool
}

// artifactScan은 줄 단위 이벤트를 읽으며 호출·응답 짝을 맞추는 상태를 든다.
type artifactScan struct {
	facts       *artifactFacts
	claudeCalls map[string]bool
	codexCalls  map[int]bool
	requests    int
}

func readArtifactFacts(path string) (*artifactFacts, bool) {
	file, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer file.Close()
	scan := &artifactScan{
		facts:       &artifactFacts{revisions: map[string]bool{}},
		claudeCalls: map[string]bool{},
		codexCalls:  map[int]bool{},
	}
	scanner := bufio.NewScanner(io.LimitReader(file, maxArtifactBytes))
	scanner.Buffer(make([]byte, 0, 64<<10), maxArtifactBytes)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var event map[string]any
		if json.Unmarshal(line, &event) != nil {
			continue
		}
		scan.consume(event)
	}
	return scan.facts, true
}

func (scan *artifactScan) consume(event map[string]any) {
	switch {
	case event["direction"] != nil:
		scan.consumeCodexWire(event)
	case event["type"] != nil:
		scan.consumeClaudeStream(event)
	case event["step"] != nil:
		scan.consumeCodexStep(event)
	default:
		scan.consumeOmoTransport(event)
	}
}

// consumeCodexWire는 Codex app-server 기록(`direction` + JSON-RPC `raw`)을 읽는다. 요청 줄에는
// id가 없으므로 client->host 요청의 순서를 JSON-RPC id로 대응시키고, docs_index 결과는 응답
// 본문에 docs 필드가 있는지로 알아본다.
func (scan *artifactScan) consumeCodexWire(event map[string]any) {
	switch event["direction"] {
	case "client->host":
		scan.requests++
		if event["method"] == "mcpServer/tool/call" {
			scan.codexCalls[scan.requests] = true
		}
	case "host->client":
		raw, _ := event["raw"].(string)
		var message map[string]any
		if json.Unmarshal([]byte(raw), &message) != nil {
			return
		}
		scan.consumeJSONRPCResponse(message)
	}
}

func (scan *artifactScan) consumeJSONRPCResponse(message map[string]any) {
	id, hasID := message["id"].(float64)
	call := hasID && scan.codexCalls[int(id)]
	if _, failed := message["error"]; failed {
		if call {
			scan.facts.docsFailed = true
		}
		return
	}
	result, _ := message["result"].(map[string]any)
	if result == nil {
		return
	}
	if revision, _ := result["protocolVersion"].(string); revision != "" {
		scan.facts.revisions[revision] = true
	}
	data, _ := result["data"].([]any)
	for _, item := range data {
		server, _ := item.(map[string]any)
		tools, _ := server["tools"].(map[string]any)
		if _, listed := tools["docs_index"]; listed {
			scan.facts.catalogListsDocs = true
		}
	}
	if call {
		scan.consumeCodexToolResult(result)
	}
}

func (scan *artifactScan) consumeCodexToolResult(result map[string]any) {
	if failed, _ := result["isError"].(bool); failed {
		scan.facts.docsFailed = true
		return
	}
	content, _ := result["content"].([]any)
	for _, item := range content {
		entry, _ := item.(map[string]any)
		text, _ := entry["text"].(string)
		var payload map[string]any
		if json.Unmarshal([]byte(text), &payload) != nil {
			continue
		}
		if ok, present := payload["ok"].(bool); present && !ok {
			scan.facts.docsFailed = true
			return
		}
		if _, present := payload["docs"]; present {
			scan.facts.docsSucceeded = true
		}
	}
}

// consumeClaudeStream은 Claude stream-json의 init(tools), tool_use, tool_result(is_error)를 읽는다.
func (scan *artifactScan) consumeClaudeStream(event map[string]any) {
	switch event["type"] {
	case "system":
		if event["subtype"] != "init" {
			return
		}
		tools, _ := event["tools"].([]any)
		for _, tool := range tools {
			if name, _ := tool.(string); isDocsIndexTool(name) {
				scan.facts.catalogListsDocs = true
			}
		}
	case "assistant", "user":
		message, _ := event["message"].(map[string]any)
		content, _ := message["content"].([]any)
		for _, item := range content {
			block, _ := item.(map[string]any)
			switch block["type"] {
			case "tool_use":
				name, _ := block["name"].(string)
				id, _ := block["id"].(string)
				if isDocsIndexTool(name) && id != "" {
					scan.claudeCalls[id] = true
				}
			case "tool_result":
				id, _ := block["tool_use_id"].(string)
				if !scan.claudeCalls[id] {
					continue
				}
				if failed, _ := block["is_error"].(bool); failed {
					scan.facts.docsFailed = true
				} else {
					scan.facts.docsSucceeded = true
				}
			}
		}
	}
}

func isDocsIndexTool(name string) bool {
	return name == "docs_index" || strings.HasSuffix(name, "__docs_index")
}

// consumeCodexStep은 Codex QA 실행이 남긴 inventory/tool 요약 줄을 읽는다.
func (scan *artifactScan) consumeCodexStep(event map[string]any) {
	switch event["step"] {
	case "inventory":
		servers, _ := event["servers"].([]any)
		for _, item := range servers {
			server, _ := item.(map[string]any)
			if listsDocsIndex(server["tools"]) {
				scan.facts.catalogListsDocs = true
			}
		}
	case "tool":
		if event["tool"] != "docs_index" {
			return
		}
		ok, present := event["ok"].(bool)
		isError, _ := event["isError"].(bool)
		switch {
		case isError || (present && !ok):
			scan.facts.docsFailed = true
		case present:
			scan.facts.docsSucceeded = true
		}
	}
}

// consumeOmoTransport는 Omo transport run 한 줄(ok, docs, tools, 협상 revision 배열)을 읽는다.
func (scan *artifactScan) consumeOmoTransport(event map[string]any) {
	ok, present := event["ok"].(bool)
	if !present {
		return
	}
	_, hasDocs := event["docs"].(float64)
	isError, _ := event["is_error"].(bool)
	isErrorCamel, _ := event["isError"].(bool)
	if !ok || isError || isErrorCamel {
		if hasDocs {
			scan.facts.docsFailed = true
		}
		return
	}
	if hasDocs {
		scan.facts.docsSucceeded = true
	}
	if listed, _ := event["has_docs_index"].(bool); listed || listsDocsIndex(event["tools"]) {
		scan.facts.catalogListsDocs = true
	}
	for _, field := range []string{"revisions", "mcp_protocol_version_headers"} {
		revisions, _ := event[field].([]any)
		for _, item := range revisions {
			if revision, _ := item.(string); revision != "" {
				scan.facts.revisions[revision] = true
			}
		}
	}
}

func listsDocsIndex(tools any) bool {
	names, _ := tools.([]any)
	for _, item := range names {
		if name, _ := item.(string); name == "docs_index" {
			return true
		}
	}
	return false
}

func (vetter *receiptVetter) staleReason(observed inspectcontract.Observation) string {
	if vetter.receipt.Transport != vetter.transport || vetter.transport == "" {
		return "receipt_stale_transport"
	}
	if vetter.receipt.ConfigPath == "" || filepath.Clean(vetter.receipt.ConfigPath) != filepath.Clean(vetter.configPath) {
		return "receipt_stale_config_path"
	}
	if observed.ConfigSHA256 == "" {
		return "receipt_config_sha_missing"
	}
	if observed.ConfigSHA256 != vetter.configSHA {
		return "receipt_stale_config"
	}
	if observed.HostVersion == "" {
		return "receipt_host_version_missing"
	}
	current := vetter.hostVersion()
	if current == "" {
		return "host_version_unobservable"
	}
	if normalizeVersion(observed.HostVersion) != current {
		return "receipt_stale_host_version"
	}
	return ""
}

func (vetter *receiptVetter) hostVersion() string {
	if !vetter.versionProbed {
		vetter.versionProbed = true
		if vetter.observer.HostVersion != nil {
			vetter.currentVersion = normalizeVersion(vetter.observer.HostVersion(vetter.host))
		}
	}
	return vetter.currentVersion
}

func (vetter *receiptVetter) artifact(source string) (string, string) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" || schemePattern.MatchString(trimmed) {
		return source, "receipt_source_not_artifact"
	}
	if strings.Contains(strings.ToLower(filepath.Base(trimmed)), "synthetic") {
		return source, "receipt_source_synthetic"
	}
	path := trimmed
	if !filepath.IsAbs(path) {
		path = filepath.Join(vetter.receiptDir, path)
	}
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
		return source, "receipt_source_not_artifact"
	}
	return path, ""
}

func demote(observed inspectcontract.Observation, reason string) inspectcontract.Observation {
	observed.Status = inspectcontract.ObservationUnknown
	observed.Reason = reason
	return observed
}

func normalizeVersion(raw string) string {
	if match := versionPattern.FindString(raw); match != "" {
		return match
	}
	return strings.TrimSpace(raw)
}

// CommandHostVersion은 허용된 host 실행 파일의 `--version` 출력에서 현재 host
// 버전을 읽는다. receipt 신선도 검사에서만 호출하며 실행 파일을 찾지 못하거나
// 실패하면 빈 문자열을 돌려준다.
func CommandHostVersion(host string) string {
	name, ok := hostBinaryNames[host]
	if !ok {
		return ""
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), hostVersionTimeout)
	defer cancel()
	command := exec.CommandContext(ctx, path, "--version")
	command.WaitDelay = time.Second
	output, err := command.Output()
	if err != nil {
		return ""
	}
	if len(output) > hostVersionMaxBytes {
		output = output[:hostVersionMaxBytes]
	}
	return normalizeVersion(string(output))
}
