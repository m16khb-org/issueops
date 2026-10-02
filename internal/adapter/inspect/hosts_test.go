package inspect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	inspectcontract "issueops/internal/contract/inspect"
)

const (
	testSkill      = "atomic-commit-push"
	fixedNow       = "2026-10-02T09:00:00Z"
	bearerSecret   = "bearer-secret-value"
	codexHTTPBlock = "[mcp_servers.issueops]\nurl = \"http://127.0.0.1:47831/mcp\"\nhttp_headers = { Authorization = \"Bearer " + bearerSecret + "\" }\n"
)

type hostFixture struct {
	root, home, codexHome string
	versions              map[string]string
}

func newHostFixture(t *testing.T) hostFixture {
	t.Helper()
	base := t.TempDir()
	fixture := hostFixture{root: filepath.Join(base, "root"), home: filepath.Join(base, "home"), versions: map[string]string{}}
	fixture.codexHome = filepath.Join(fixture.home, ".codex")
	writeTestFile(t, filepath.Join(fixture.root, "skills", testSkill, "SKILL.md"), "# skill\n")
	for _, skills := range []string{
		filepath.Join(fixture.codexHome, "skills"),
		filepath.Join(fixture.home, ".claude", "skills"),
		filepath.Join(fixture.home, ".omo", "agent", "skills"),
	} {
		if err := os.MkdirAll(skills, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(fixture.root, "skills", testSkill), filepath.Join(skills, testSkill)); err != nil {
			t.Fatal(err)
		}
	}
	fixture.writeHTTPConfigs(t, "http://127.0.0.1:47831/mcp", bearerSecret)
	return fixture
}

func (fixture hostFixture) writeHTTPConfigs(t *testing.T, url, bearer string) {
	t.Helper()
	writeTestFile(t, filepath.Join(fixture.codexHome, "config.toml"),
		"model = \"gpt-test\"\n\n[mcp_servers.other]\ncommand = \"other\"\n\n[mcp_servers.issueops]\nurl = \""+url+"\"\nhttp_headers = { Authorization = \"Bearer "+bearer+"\" }\n")
	entry := `{"type":"http","url":"` + url + `","headers":{"Authorization":"Bearer ` + bearer + `"}}`
	writeTestFile(t, filepath.Join(fixture.home, ".claude.json"), `{"numStartups":3,"mcpServers":{"other":{"command":"other"},"issueops":`+entry+`}}`)
	writeTestFile(t, filepath.Join(fixture.home, ".omo", "mcp.json"), `{"mcpServers":{"issueops":`+entry+`}}`)
}

func (fixture hostFixture) observer(root string) Observer {
	fixed, err := time.Parse(time.RFC3339, fixedNow)
	if err != nil {
		panic(err)
	}
	return Observer{
		ListDocs:    func(string) []string { return nil },
		Now:         func() time.Time { return fixed },
		HostVersion: func(host string) string { return fixture.versions[host] },
		ReceiptRoot: root,
	}
}

func (fixture hostFixture) inspect(t *testing.T, options inspectcontract.Options) map[string]inspectcontract.HostIntegration {
	t.Helper()
	info := fixture.observer("").Inspect(fixture.root, fixture.root, fixture.home, "test", testSkill, options)
	hosts := map[string]inspectcontract.HostIntegration{}
	for _, host := range info.Integration.Hosts {
		hosts[host.Host] = host
	}
	if len(hosts) != 3 || len(info.Integration.Hosts) != 3 {
		t.Fatalf("expected codex, claude, omo exactly once: %+v", info.Integration.Hosts)
	}
	return hosts
}

func writeTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func requireStatus(t *testing.T, label string, observed inspectcontract.Observation, status, reason string) {
	t.Helper()
	if observed.Status != status || observed.Reason != reason {
		t.Fatalf("%s = %s/%s, want %s/%s (%+v)", label, observed.Status, observed.Reason, status, reason, observed)
	}
}

func TestHostsDefaultChecksFilesOnlyAndLeavesLiveObservationsNotChecked(t *testing.T) {
	fixture := newHostFixture(t)
	hosts := fixture.inspect(t, inspectcontract.Options{})

	for name, host := range hosts {
		requireStatus(t, name+" installed", host.Installed, "verified", "")
		requireStatus(t, name+" linked", host.Linked, "verified", "")
		requireStatus(t, name+" configured", host.Configured, "verified", "")
		requireStatus(t, name+" discovered", host.Discovered, "not_checked", "host_receipt_required")
		requireStatus(t, name+" connected", host.Connected, "not_checked", "host_receipt_required")
		requireStatus(t, name+" protocol", host.Protocol, "not_checked", "host_receipt_required")
		if host.Transport != "http" || len(host.Configured.ConfigSHA256) != 64 || host.Configured.ObservedAt != fixedNow || host.Configured.Source != host.ConfigPath {
			t.Fatalf("%s lacks evidence: %+v", name, host)
		}
	}
	if hosts["codex"].ConfigPath != filepath.Join(fixture.codexHome, "config.toml") ||
		hosts["claude"].ConfigPath != filepath.Join(fixture.home, ".claude.json") ||
		hosts["omo"].SkillPath != filepath.Join(fixture.home, ".omo", "agent", "skills", testSkill) {
		t.Fatalf("unexpected host paths: %+v", hosts)
	}
}

func TestHostsDefaultJSONKeepsExistingFieldsAndNeverLeaksSecrets(t *testing.T) {
	fixture := newHostFixture(t)
	info := fixture.observer("").Inspect(fixture.root, fixture.root, fixture.home, "test", testSkill, inspectcontract.Options{})
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	integration := decoded["integration"].(map[string]any)
	for _, key := range []string{"codex_skill_path", "codex_skill_installed", "codex_mcp_configured", "claude_skill_path", "claude_skill_installed",
		"project_claude_skill_path", "project_claude_skill", "project_claude_mcp_config", "mcp_binary_path", "hosts"} {
		if _, ok := integration[key]; !ok {
			t.Fatalf("integration lost %s: %s", key, encoded)
		}
	}
	for _, key := range []string{"installed", "linked", "configured", "discovered", "connected", "protocol"} {
		observed := integration["hosts"].([]any)[0].(map[string]any)[key].(map[string]any)
		for _, field := range []string{"status", "source", "observed_at", "reason", "host_version", "requested_revision", "negotiated_revision", "config_sha256", "features"} {
			if _, ok := observed[field]; !ok {
				t.Fatalf("%s lacks %s", key, field)
			}
		}
		if _, isNull := observed["features"].([]any); !isNull {
			t.Fatalf("%s features must be an array, got %#v", key, observed["features"])
		}
	}
	if strings.Contains(string(encoded), bearerSecret) {
		t.Fatalf("bearer leaked into inspect output: %s", encoded)
	}
}

func TestHostsConfigHashIgnoresSecretsButTracksEndpoint(t *testing.T) {
	fixture := newHostFixture(t)
	before := fixture.inspect(t, inspectcontract.Options{})
	fixture.writeHTTPConfigs(t, "http://127.0.0.1:47831/mcp", "rotated-secret")
	rotated := fixture.inspect(t, inspectcontract.Options{})
	fixture.writeHTTPConfigs(t, "http://127.0.0.1:47999/mcp", "rotated-secret")
	moved := fixture.inspect(t, inspectcontract.Options{})

	for _, name := range []string{"codex", "claude", "omo"} {
		if before[name].Configured.ConfigSHA256 != rotated[name].Configured.ConfigSHA256 {
			t.Fatalf("%s: bearer rotation changed the hash", name)
		}
		if before[name].Configured.ConfigSHA256 == moved[name].Configured.ConfigSHA256 {
			t.Fatalf("%s: endpoint change kept the hash", name)
		}
	}
}

func TestHostsCodexHomeOptionRelocatesCodexConfigAndSkills(t *testing.T) {
	fixture := newHostFixture(t)
	custom := filepath.Join(t.TempDir(), "custom-codex")
	if err := os.MkdirAll(filepath.Join(custom, "skills"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(fixture.root, "skills", testSkill), filepath.Join(custom, "skills", testSkill)); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(custom, "config.toml"), "[mcp_servers.issueops]\ncommand = \"/bin/issueops\"\nargs = [\"mcp\"]\nstartup_timeout_sec = 30\n\n[mcp_servers.issueops.env]\nISSUEOPS_ROOT = \"/r\"\n")

	codex := fixture.inspect(t, inspectcontract.Options{CodexHome: custom})["codex"]

	if codex.ConfigPath != filepath.Join(custom, "config.toml") || codex.SkillPath != filepath.Join(custom, "skills", testSkill) {
		t.Fatalf("codex paths ignored CodexHome: %+v", codex)
	}
	requireStatus(t, "configured", codex.Configured, "verified", "")
	if codex.Transport != "stdio" {
		t.Fatalf("transport = %q, want stdio", codex.Transport)
	}
}

func TestHostsBrokenSymlinkFailsInstalledAndLinked(t *testing.T) {
	fixture := newHostFixture(t)
	skillPath := filepath.Join(fixture.home, ".claude", "skills", testSkill)
	if err := os.Remove(skillPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(fixture.root, "skills", "removed-skill"), skillPath); err != nil {
		t.Fatal(err)
	}

	claude := fixture.inspect(t, inspectcontract.Options{})["claude"]

	requireStatus(t, "installed", claude.Installed, "failed", "broken_symlink")
	requireStatus(t, "linked", claude.Linked, "failed", "broken_symlink")
	requireStatus(t, "configured", claude.Configured, "verified", "")
}

func TestHostsDistinguishCopiedWrongTargetAndMissingSkill(t *testing.T) {
	fixture := newHostFixture(t)
	copied := filepath.Join(fixture.codexHome, "skills", testSkill)
	if err := os.Remove(copied); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(copied, "SKILL.md"), "# copy\n")
	other := filepath.Join(t.TempDir(), "other-skill")
	writeTestFile(t, filepath.Join(other, "SKILL.md"), "# other\n")
	claudeLink := filepath.Join(fixture.home, ".claude", "skills", testSkill)
	if err := os.Remove(claudeLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(other, claudeLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(fixture.home, ".omo", "agent", "skills", testSkill)); err != nil {
		t.Fatal(err)
	}

	hosts := fixture.inspect(t, inspectcontract.Options{})

	requireStatus(t, "codex installed", hosts["codex"].Installed, "verified", "")
	requireStatus(t, "codex linked", hosts["codex"].Linked, "failed", "not_symlink")
	requireStatus(t, "claude installed", hosts["claude"].Installed, "verified", "")
	requireStatus(t, "claude linked", hosts["claude"].Linked, "failed", "link_target_mismatch")
	requireStatus(t, "omo installed", hosts["omo"].Installed, "failed", "skill_missing")
	requireStatus(t, "omo linked", hosts["omo"].Linked, "failed", "skill_missing")
}

func TestHostsConfigFailuresCarryDistinctReasons(t *testing.T) {
	cases := []struct {
		name   string
		codex  string
		json   string
		reason string
	}{
		{"missing file", "", "", "config_missing"},
		{"malformed", "[mcp_servers.issueops\nurl = \"http://x\"\n", `{"mcpServers":`, "config_malformed"},
		{"bad value", "[mcp_servers.issueops]\nurl = nope\n", `{"mcpServers":{"issueops":"text"}}`, ""},
		{"no entry", "model = \"x\"\n", `{"mcpServers":{"other":{"command":"o"}}}`, "entry_missing"},
		{"duplicate", codexHTTPBlock + "\n" + codexHTTPBlock,
			`{"mcpServers":{"issueops":{"type":"http","url":"http://a"},"issueops":{"type":"http","url":"http://b"}}}`, "entry_duplicate"},
		{"no transport", "[mcp_servers.issueops]\nstartup_timeout_sec = 3\n", `{"mcpServers":{"issueops":{}}}`, "entry_transport_unknown"},
		{"both transports", "[mcp_servers.issueops]\nurl = \"http://x\"\ncommand = \"c\"\n", `{"mcpServers":{"issueops":{"url":"http://x","command":"c"}}}`, "entry_transport_ambiguous"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newHostFixture(t)
			if tc.name == "missing file" {
				for _, path := range []string{filepath.Join(fixture.codexHome, "config.toml"), filepath.Join(fixture.home, ".claude.json"), filepath.Join(fixture.home, ".omo", "mcp.json")} {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				writeTestFile(t, filepath.Join(fixture.codexHome, "config.toml"), tc.codex)
				writeTestFile(t, filepath.Join(fixture.home, ".claude.json"), tc.json)
				writeTestFile(t, filepath.Join(fixture.home, ".omo", "mcp.json"), tc.json)
			}

			hosts := fixture.inspect(t, inspectcontract.Options{})

			wantCodex, wantJSON := tc.reason, tc.reason
			if tc.name == "bad value" {
				wantCodex, wantJSON = "config_malformed", "entry_malformed"
			}
			requireStatus(t, "codex configured", hosts["codex"].Configured, "failed", wantCodex)
			requireStatus(t, "claude configured", hosts["claude"].Configured, "failed", wantJSON)
			requireStatus(t, "omo configured", hosts["omo"].Configured, "failed", wantJSON)
			requireStatus(t, "claude installed", hosts["claude"].Installed, "verified", "")
			if hosts["codex"].Configured.ConfigSHA256 != "" {
				t.Fatalf("failed configuration must not carry a hash: %+v", hosts["codex"].Configured)
			}
		})
	}
}

func TestHostsCodexParserKeepsStdioEnvAndSecretsOutOfHash(t *testing.T) {
	fixture := newHostFixture(t)
	block := func(root string) string {
		return "# managed\n[mcp_servers.issueops] # entry\ncommand = \"/bin/issueops\"\nargs = [\"mcp\"]\nstartup_timeout_sec = 30\n\n[mcp_servers.issueops.env]\nISSUEOPS_ROOT = '" + root + "'\nTOKEN = \"s3cret\"\n"
	}
	writeTestFile(t, filepath.Join(fixture.codexHome, "config.toml"), block("/one"))
	one := fixture.inspect(t, inspectcontract.Options{})["codex"]
	writeTestFile(t, filepath.Join(fixture.codexHome, "config.toml"), block("/two"))
	two := fixture.inspect(t, inspectcontract.Options{})["codex"]

	requireStatus(t, "configured", one.Configured, "verified", "")
	if one.Transport != "stdio" || one.Configured.ConfigSHA256 != two.Configured.ConfigSHA256 {
		t.Fatalf("env values must be redacted from the hash: %+v %+v", one, two)
	}
}

type receiptFixture struct {
	dir      string
	artifact string
}

func newReceiptFixture(t *testing.T) receiptFixture {
	t.Helper()
	dir := t.TempDir()
	artifact := filepath.Join(dir, "host-run-stream.jsonl")
	writeTestFile(t, artifact, `{"host":"omo-installed-native-transport","url":"http://127.0.0.1:47831/mcp","tools":["api_doc_review","docs_index"],"ok":true,"docs":3,"mcp_protocol_version_headers":["2025-11-25"],"is_error":false}`+"\n")
	return receiptFixture{dir: dir, artifact: artifact}
}

func (receipts receiptFixture) hostReceipt(current inspectcontract.HostIntegration, version string) inspectcontract.HostIntegration {
	evidence := func(features ...string) inspectcontract.Observation {
		return inspectcontract.Observation{
			Status: "verified", Source: receipts.artifact, ObservedAt: "2026-10-02T08:00:00Z", HostVersion: version,
			ConfigSHA256: current.Configured.ConfigSHA256, Features: features, RequestedRevision: "2025-11-25", NegotiatedRevision: "2025-11-25",
		}
	}
	return inspectcontract.HostIntegration{
		Host: current.Host, Transport: current.Transport, ConfigPath: current.ConfigPath,
		Discovered: evidence("tools/list"), Connected: evidence("tools/list", "docs_index"), Protocol: evidence("initialize"),
	}
}

func (receipts receiptFixture) write(t *testing.T, hosts ...inspectcontract.HostIntegration) string {
	t.Helper()
	body, err := json.Marshal(inspectcontract.HostReceipts{SchemaVersion: 1, Hosts: hosts})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(receipts.dir, "receipts.json")
	writeTestFile(t, path, string(body))
	return path
}

func TestHostsReceiptVerifiesDiscoveredConnectedProtocolFromRealArtifact(t *testing.T) {
	fixture := newHostFixture(t)
	fixture.versions["codex"] = "codex-cli 0.128.0"
	receipts := newReceiptFixture(t)
	base := fixture.inspect(t, inspectcontract.Options{})
	path := receipts.write(t, receipts.hostReceipt(base["codex"], "0.128.0"))

	hosts := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})

	codex := hosts["codex"]
	requireStatus(t, "discovered", codex.Discovered, "verified", "")
	requireStatus(t, "connected", codex.Connected, "verified", "")
	requireStatus(t, "protocol", codex.Protocol, "verified", "")
	if codex.Connected.Source != receipts.artifact || codex.Connected.HostVersion != "0.128.0" || codex.Protocol.NegotiatedRevision != "2025-11-25" ||
		!contains(codex.Connected.Features, "docs_index") {
		t.Fatalf("evidence was not preserved: %+v", codex.Connected)
	}
	requireStatus(t, "claude has no receipt", hosts["claude"].Connected, "not_checked", "no_receipt_for_host")
	requireStatus(t, "file states unchanged", codex.Configured, "verified", "")
}

func TestHostsReceiptStalenessDemotesVerifiedToUnknown(t *testing.T) {
	mutations := []struct {
		name   string
		mutate func(*testing.T, hostFixture, *inspectcontract.HostIntegration)
		reason string
	}{
		{"host version changed", func(_ *testing.T, f hostFixture, _ *inspectcontract.HostIntegration) { f.versions["codex"] = "0.129.0" }, "receipt_stale_host_version"},
		{"host version unobservable", func(_ *testing.T, f hostFixture, _ *inspectcontract.HostIntegration) { delete(f.versions, "codex") }, "host_version_unobservable"},
		{"config changed", func(t *testing.T, f hostFixture, _ *inspectcontract.HostIntegration) {
			f.writeHTTPConfigs(t, "http://127.0.0.1:47999/mcp", bearerSecret)
		}, "receipt_stale_config"},
		{"transport changed", func(_ *testing.T, _ hostFixture, receipt *inspectcontract.HostIntegration) {
			receipt.Transport = "stdio"
		}, "receipt_stale_transport"},
		{"config hash missing", func(_ *testing.T, _ hostFixture, r *inspectcontract.HostIntegration) { r.Connected.ConfigSHA256 = "" }, "receipt_config_sha_missing"},
		{"receipt version missing", func(_ *testing.T, _ hostFixture, r *inspectcontract.HostIntegration) { r.Connected.HostVersion = "" }, "receipt_host_version_missing"},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newHostFixture(t)
			fixture.versions["codex"] = "0.128.0"
			receipts := newReceiptFixture(t)
			receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
			tc.mutate(t, fixture, &receipt)
			path := receipts.write(t, receipt)

			codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

			requireStatus(t, "connected", codex.Connected, "unknown", tc.reason)
		})
	}
}

func TestHostsReceiptSyntheticOrMissingSourceNeverVerifies(t *testing.T) {
	sources := map[string]string{
		"scheme":         "synthetic:docs_index",
		"url":            "https://example.test/run.json",
		"empty":          "",
		"missing file":   "/nonexistent/host-run.jsonl",
		"synthetic name": "SYNTHETIC-host-run.jsonl",
	}
	for name, source := range sources {
		t.Run(name, func(t *testing.T) {
			fixture := newHostFixture(t)
			fixture.versions["codex"] = "0.128.0"
			receipts := newReceiptFixture(t)
			writeTestFile(t, filepath.Join(receipts.dir, "SYNTHETIC-host-run.jsonl"), "{}\n")
			receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
			if source == "SYNTHETIC-host-run.jsonl" {
				source = filepath.Join(receipts.dir, source)
			}
			receipt.Connected.Source = source
			path := receipts.write(t, receipt)

			codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

			if codex.Connected.Status != "unknown" || !strings.HasPrefix(codex.Connected.Reason, "receipt_source_") {
				t.Fatalf("connected = %+v", codex.Connected)
			}
		})
	}
}

func TestHostsReceiptRelativeSourceResolvesNextToReceiptFile(t *testing.T) {
	fixture := newHostFixture(t)
	fixture.versions["codex"] = "0.128.0"
	receipts := newReceiptFixture(t)
	receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
	receipt.Connected.Source = "host-run-stream.jsonl"
	path := receipts.write(t, receipt)

	codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

	requireStatus(t, "connected", codex.Connected, "verified", "")
	if codex.Connected.Source != receipts.artifact {
		t.Fatalf("source = %q, want %q", codex.Connected.Source, receipts.artifact)
	}
}

func TestHostsReceiptRequiresEvidenceThatMatchesTheObservation(t *testing.T) {
	fixture := newHostFixture(t)
	fixture.versions["codex"] = "0.128.0"
	receipts := newReceiptFixture(t)
	receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
	receipt.Discovered.Features = []string{"resources/list"}
	receipt.Connected.Features = []string{"tools/list"}
	receipt.Protocol.RequestedRevision, receipt.Protocol.NegotiatedRevision = "", ""
	path := receipts.write(t, receipt)

	codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

	requireStatus(t, "discovered", codex.Discovered, "unknown", "receipt_missing_tools_list")
	requireStatus(t, "connected", codex.Connected, "unknown", "receipt_missing_docs_index")
	requireStatus(t, "protocol", codex.Protocol, "unknown", "receipt_missing_revision")
}

func TestHostsReceiptMustBindArtifactContentAndConfigPathToTheCurrentHost(t *testing.T) {
	cases := []struct {
		name                    string
		mutate                  func(*testing.T, receiptFixture, *inspectcontract.HostIntegration)
		listReason, protoReason string
	}{
		{"unrelated regular file", func(t *testing.T, receipts receiptFixture, r *inspectcontract.HostIntegration) {
			unrelated := filepath.Join(receipts.dir, "hosts")
			writeTestFile(t, unrelated, "127.0.0.1 localhost\n")
			r.Discovered.Source, r.Connected.Source, r.Protocol.Source = unrelated, unrelated, unrelated
		}, "receipt_artifact_missing_evidence", "receipt_artifact_missing_revision"},
		{"other environment", func(_ *testing.T, _ receiptFixture, r *inspectcontract.HostIntegration) {
			r.ConfigPath = "/other-home/.codex/config.toml"
		}, "receipt_stale_config_path", "receipt_stale_config_path"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fixture := newHostFixture(t)
			fixture.versions["codex"] = "0.128.0"
			receipts := newReceiptFixture(t)
			receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
			tc.mutate(t, receipts, &receipt)
			path := receipts.write(t, receipt)

			codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

			requireStatus(t, "discovered", codex.Discovered, "unknown", tc.listReason)
			requireStatus(t, "connected", codex.Connected, "unknown", tc.listReason)
			requireStatus(t, "protocol", codex.Protocol, "unknown", tc.protoReason)
		})
	}
}

func TestHostsReceiptFailedObservationFromRealArtifactStaysFailed(t *testing.T) {
	fixture := newHostFixture(t)
	fixture.versions["codex"] = "0.128.0"
	receipts := newReceiptFixture(t)
	receipt := receipts.hostReceipt(fixture.inspect(t, inspectcontract.Options{})["codex"], "0.128.0")
	receipt.Connected.Status, receipt.Connected.Reason = "failed", "docs_index_rejected"
	path := receipts.write(t, receipt)

	codex := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})["codex"]

	requireStatus(t, "connected", codex.Connected, "failed", "docs_index_rejected")
}

func TestHostsReceiptFileProblemsMarkEveryLiveObservationUnknown(t *testing.T) {
	cases := map[string]struct {
		body   string
		reason string
	}{
		"malformed":      {"{", "receipt_malformed"},
		"unknown field":  {`{"schema_version":1,"hosts":[],"extra":true}`, "receipt_malformed"},
		"future schema":  {`{"schema_version":2,"hosts":[]}`, "receipt_schema_unsupported"},
		"missing schema": {`{"hosts":[]}`, "receipt_schema_unsupported"},
		"duplicate host": {`{"schema_version":1,"hosts":[{"host":"codex"},{"host":"codex"}]}`, "receipt_duplicate_host"},
		"trailing data":  {`{"schema_version":1,"hosts":[]} {}`, "receipt_malformed"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fixture := newHostFixture(t)
			path := filepath.Join(t.TempDir(), "receipts.json")
			writeTestFile(t, path, tc.body)

			hosts := fixture.inspect(t, inspectcontract.Options{HostReceipts: path})

			for host, integration := range hosts {
				requireStatus(t, host+" discovered", integration.Discovered, "unknown", tc.reason)
				requireStatus(t, host+" connected", integration.Connected, "unknown", tc.reason)
				requireStatus(t, host+" protocol", integration.Protocol, "unknown", tc.reason)
				requireStatus(t, host+" configured", integration.Configured, "verified", "")
			}
		})
	}
}

func TestHostsReceiptUnreadableFileIsUnknownNotAnInspectFailure(t *testing.T) {
	fixture := newHostFixture(t)

	info := fixture.observer("").Inspect(fixture.root, fixture.root, fixture.home, "test", testSkill,
		inspectcontract.Options{HostReceipts: filepath.Join(t.TempDir(), "absent.json")})

	if !info.OK {
		t.Fatalf("inspect must still succeed: %+v", info)
	}
	requireStatus(t, "connected", info.Integration.Hosts[0].Connected, "unknown", "receipt_unreadable")
}

func TestHostsReceiptRootConfinesReceiptFiles(t *testing.T) {
	fixture := newHostFixture(t)
	workspace := t.TempDir()
	inside := filepath.Join(workspace, "receipts.json")
	writeTestFile(t, inside, `{"schema_version":1,"hosts":[]}`)
	outside := filepath.Join(t.TempDir(), "receipts.json")
	writeTestFile(t, outside, `{"schema_version":1,"hosts":[]}`)
	link := filepath.Join(workspace, "escape.json")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	observer := fixture.observer(workspace)
	inspectWith := func(path string) inspectcontract.HostIntegration {
		return observer.Inspect(fixture.root, fixture.root, fixture.home, "test", testSkill, inspectcontract.Options{HostReceipts: path}).Integration.Hosts[0]
	}

	requireStatus(t, "relative inside", inspectWith("receipts.json").Connected, "not_checked", "no_receipt_for_host")
	requireStatus(t, "absolute outside", inspectWith(outside).Connected, "unknown", "receipt_outside_workspace")
	requireStatus(t, "symlink escape", inspectWith(link).Connected, "unknown", "receipt_outside_workspace")
	requireStatus(t, "dotdot", inspectWith("../"+filepath.Base(filepath.Dir(outside))+"/receipts.json").Connected, "unknown", "receipt_outside_workspace")
}

func TestNormalizeVersionExtractsHostVersionFromBanner(t *testing.T) {
	for raw, want := range map[string]string{
		"codex-cli 0.128.0\n":        "0.128.0",
		"2.1.287 (Claude Code)":      "2.1.287",
		"omo 1.4.2-beta.1+build7 ok": "1.4.2-beta.1+build7",
		"  dev  ":                    "dev",
		"":                           "",
	} {
		if got := normalizeVersion(raw); got != want {
			t.Fatalf("normalizeVersion(%q) = %q, want %q", raw, got, want)
		}
	}
}

func TestCommandHostVersionRejectsUnknownHostsAndMissingBinaries(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if got := CommandHostVersion("agy"); got != "" {
		t.Fatalf("unknown host = %q", got)
	}
	if got := CommandHostVersion("codex"); got != "" {
		t.Fatalf("missing binary = %q", got)
	}
}

func TestCommandHostVersionReadsVersionFromAllowedBinary(t *testing.T) {
	bin := t.TempDir()
	writeTestFile(t, filepath.Join(bin, "codex"), "#!/bin/sh\necho 'codex-cli 9.8.7'\n")
	if err := os.Chmod(filepath.Join(bin, "codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)

	if got := CommandHostVersion("codex"); got != "9.8.7" {
		t.Fatalf("version = %q, want 9.8.7", got)
	}
}
