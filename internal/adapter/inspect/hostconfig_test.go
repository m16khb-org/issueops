package inspect

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseTOMLValueHandlesTypesAndRejectsBrokenValues(t *testing.T) {
	good := []struct {
		name, text string
		want       any
	}{
		{"basic string with escape", `"a\"b\n"`, "a\"b\n"},
		{"literal string keeps backslash", `'C:\x'`, `C:\x`},
		{"true", "true", true},
		{"false", "false", false},
		{"integer with underscores", "1_000", int64(1000)},
		{"hex integer", "0x10", int64(16)},
		{"float", "1.5", 1.5},
		{"array with trailing comma", `["a", 'b', 3,]`, []any{"a", "b", int64(3)}},
		{"empty array", "[]", []any{}},
		{"nested inline table", `{ a = 1, "b c" = { d = true } }`, map[string]any{"a": int64(1), "b c": map[string]any{"d": true}}},
		{"surrounding spaces", "  \t7  ", int64(7)},
	}
	for _, tc := range good {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTOMLValue(tc.text)
			if err != nil {
				t.Fatalf("parseTOMLValue(%q): %v", tc.text, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("parseTOMLValue(%q) = %#v, want %#v", tc.text, got, tc.want)
			}
		})
	}
	bad := map[string]string{
		"empty":                       "",
		"unterminated basic string":   `"abc`,
		"trailing escape":             `"abc\`,
		"invalid escape":              `"\q"`,
		"unterminated literal string": `'abc`,
		"unterminated array":          `[1, 2`,
		"array item error":            `[nope]`,
		"unterminated inline table":   `{ a = 1`,
		"inline key without value":    `{ a }`,
		"inline value error":          `{ a = nope }`,
		"bare word":                   "nope",
		"trailing characters":         `"a" "b"`,
	}
	for name, text := range bad {
		t.Run("rejects "+name, func(t *testing.T) {
			if got, err := parseTOMLValue(text); err == nil {
				t.Fatalf("parseTOMLValue(%q) = %#v, want error", text, got)
			}
		})
	}
}

func TestStripTOMLCommentAndBracketDepthRespectQuotes(t *testing.T) {
	comments := map[string]string{
		`a = 1 # note`:           `a = 1 `,
		`a = "x # y" # note`:     `a = "x # y" `,
		`a = 'x # y'`:            `a = 'x # y'`,
		`a = "q\"# still" # end`: `a = "q\"# still" `,
		`# whole line`:           ``,
		`a = "x"`:                `a = "x"`,
	}
	for line, want := range comments {
		if got := stripTOMLComment(line); got != want {
			t.Errorf("stripTOMLComment(%q) = %q, want %q", line, got, want)
		}
	}
	depths := map[string]int{
		`[1, 2`:            1,
		`{ a = [1, 2] }`:   0,
		`"[" `:             0,
		`'{'`:              0,
		`"a\"[" + [`:       1,
		`[ { a = 1 },`:     1,
		`1 ]`:              -1,
		`["x"], {y = 'z'}`: 0,
	}
	for text, want := range depths {
		if got := bracketDepth(text); got != want {
			t.Errorf("bracketDepth(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestParseCodexIssueOpsTableCollectsOnlyIssueOpsEntries(t *testing.T) {
	t.Run("inline entry under mcp_servers parent", func(t *testing.T) {
		text := "[mcp_servers]\nother = { command = \"o\" }\nissueops = { command = \"c\", args = [\"a\",\n  \"b\"] }\n"
		entry, count, err := parseCodexIssueOpsTable(text)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"command": "c", "args": []any{"a", "b"}}
		if count != 1 || !reflect.DeepEqual(entry, want) {
			t.Fatalf("count=%d entry=%#v, want 1 %#v", count, entry, want)
		}
	})
	t.Run("sibling and array tables are ignored", func(t *testing.T) {
		text := "[mcp_servers.other]\ncommand = \"x\"\n[[profiles]]\nname = 1\n[mcp_servers.issueops]\nurl = \"http://h\"\n[mcp_servers.issueops_extra]\nurl = \"nope\"\n"
		entry, count, err := parseCodexIssueOpsTable(text)
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 || !reflect.DeepEqual(entry, map[string]any{"url": "http://h"}) {
			t.Fatalf("count=%d entry=%#v", count, entry)
		}
	})
	t.Run("quoted table header and sub table", func(t *testing.T) {
		text := "[ mcp_servers.\"issueops\" ]\ncommand = 'c'\n[mcp_servers.issueops.env]\n\"KEY\" = \"v\"\nOTHER = 2\n"
		entry, count, err := parseCodexIssueOpsTable(text)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]any{"command": "c", "env": map[string]any{"KEY": "v", "OTHER": int64(2)}}
		if count != 1 || !reflect.DeepEqual(entry, want) {
			t.Fatalf("count=%d entry=%#v, want %#v", count, entry, want)
		}
	})
	t.Run("two headers count as duplicates", func(t *testing.T) {
		_, count, err := parseCodexIssueOpsTable("[mcp_servers.issueops]\nurl = \"a\"\n[mcp_servers.issueops]\nurl = \"b\"\n")
		if err != nil || count != 2 {
			t.Fatalf("count=%d err=%v, want 2 nil", count, err)
		}
	})
	t.Run("inline plus header count as duplicates", func(t *testing.T) {
		_, count, err := parseCodexIssueOpsTable("[mcp_servers]\nissueops = { url = \"a\" }\n[mcp_servers.issueops]\nurl = \"b\"\n")
		if err != nil || count != 2 {
			t.Fatalf("count=%d err=%v, want 2 nil", count, err)
		}
	})
	rejected := map[string]string{
		"unterminated header":         "[mcp_servers.issueops\n",
		"key without equals":          "[mcp_servers.issueops]\njust a line\n",
		"bad value in issueops table": "[mcp_servers.issueops]\nurl = nope\n",
		"inline issueops not a table": "[mcp_servers]\nissueops = \"text\"\n",
		"bad value in sub table":      "[mcp_servers.issueops.env]\nA = nope\n",
		"unclosed multi line array":   "[mcp_servers.issueops]\nargs = [\"a\",\n",
	}
	for name, text := range rejected {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, _, err := parseCodexIssueOpsTable(text); err == nil {
				t.Fatalf("parseCodexIssueOpsTable(%q) succeeded, want error", text)
			}
		})
	}
	t.Run("unrelated tables with malformed values are not evaluated", func(t *testing.T) {
		entry, count, err := parseCodexIssueOpsTable("[other]\nweird = nope\nno equals here\n")
		if err != nil || count != 0 || len(entry) != 0 {
			t.Fatalf("entry=%#v count=%d err=%v", entry, count, err)
		}
	})
	t.Run("other keys in mcp_servers parent are skipped without parsing", func(t *testing.T) {
		_, count, err := parseCodexIssueOpsTable("[mcp_servers]\nother = nope\n")
		if err != nil || count != 0 {
			t.Fatalf("count=%d err=%v", count, err)
		}
	})
}

func TestEntryTransportClassifiesEntries(t *testing.T) {
	cases := []struct {
		name   string
		entry  map[string]any
		want   string
		reason string
	}{
		{"http url", map[string]any{"url": "http://h"}, "http", ""},
		{"http explicit type", map[string]any{"url": "http://h", "type": "http"}, "http", ""},
		{"stdio command", map[string]any{"command": "c"}, "stdio", ""},
		{"stdio explicit type", map[string]any{"command": "c", "type": "stdio"}, "stdio", ""},
		{"url with stdio type", map[string]any{"url": "http://h", "type": "stdio"}, "", "entry_type_unsupported"},
		{"command with http type", map[string]any{"command": "c", "type": "http"}, "", "entry_type_unsupported"},
		{"both", map[string]any{"url": "u", "command": "c"}, "", "entry_transport_ambiguous"},
		{"neither", map[string]any{"type": "http"}, "", "entry_transport_unknown"},
		{"empty strings", map[string]any{"url": "", "command": ""}, "", "entry_transport_unknown"},
		{"non string url", map[string]any{"url": 3}, "", "entry_transport_unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := entryTransport(tc.entry)
			if tc.reason == "" {
				if err != nil || got != tc.want {
					t.Fatalf("got %q, %v; want %q", got, err, tc.want)
				}
				return
			}
			if reasonCode(err) != tc.reason {
				t.Fatalf("reason = %q (%v), want %q", reasonCode(err), err, tc.reason)
			}
		})
	}
}

func TestReasonCodeFallsBackToConfigUnreadableForUncodedErrors(t *testing.T) {
	if got := reasonCode(configReadError{"entry_missing"}); got != "entry_missing" {
		t.Fatalf("coded error reason = %q", got)
	}
	if got := (configReadError{"entry_missing"}).Error(); got != "entry_missing" {
		t.Fatalf("Error() = %q", got)
	}
	if got := reasonCode(errors.New("boom")); got != "config_unreadable" {
		t.Fatalf("uncoded error reason = %q", got)
	}
}

func TestStripSecretsRedactsSecretKeysAndHeaderEnvValuesRecursively(t *testing.T) {
	in := map[string]any{
		"url":          "http://h",
		"api_key":      "k",
		"nested":       []any{map[string]any{"Password": "p", "keep": "v"}},
		"env":          map[string]any{"A": "1", "B": "2"},
		"headers":      "not-a-table",
		"http_headers": map[string]any{"Authorization": "Bearer x"},
	}
	got, _ := stripSecrets(in).(map[string]any)
	want := map[string]any{
		"url":          "http://h",
		"api_key":      redactedValue,
		"nested":       []any{map[string]any{"Password": redactedValue, "keep": "v"}},
		"env":          map[string]any{"A": redactedValue, "B": redactedValue},
		"headers":      redactedValue,
		"http_headers": map[string]any{"Authorization": redactedValue},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("stripSecrets = %#v, want %#v", got, want)
	}
	if in["api_key"] != "k" {
		t.Fatalf("stripSecrets must not mutate its input: %#v", in)
	}
}

func TestReadJSONEntryRejectsStructuralProblemsWithDistinctReasons(t *testing.T) {
	cases := []struct {
		name, body, reason string
	}{
		{"invalid json", `{`, "config_malformed"},
		{"top level array", `[]`, "config_malformed"},
		{"mcpServers not an object", `{"mcpServers":[]}`, "config_malformed"},
		{"no mcpServers", `{"other":1}`, "entry_missing"},
		{"duplicate mcpServers", `{"mcpServers":{"issueops":{"url":"a"}},"mcpServers":{}}`, "entry_duplicate"},
		{"entry null", `{"mcpServers":{"issueops":null}}`, "entry_malformed"},
		{"entry is array", `{"mcpServers":{"issueops":[]}}`, "entry_malformed"},
		{"secret in unrelated key does not matter", `{"mcpServers":{"issueops":{"url":"http://h","token":"t"}}}`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "mcp.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o600); err != nil {
				t.Fatal(err)
			}
			entry, err := readJSONEntry(path)
			if tc.reason == "" {
				if err != nil || entry.transport != "http" || len(entry.sha256) != 64 {
					t.Fatalf("entry=%+v err=%v", entry, err)
				}
				return
			}
			if err == nil || reasonCode(err) != tc.reason {
				t.Fatalf("reason = %q (%v), want %q", reasonCode(err), err, tc.reason)
			}
		})
	}
}

func TestReadConfigEntriesDistinguishMissingFromUnreadable(t *testing.T) {
	dir := t.TempDir()
	for name, read := range map[string]func(string) (hostEntry, error){"json": readJSONEntry, "codex": readCodexEntry} {
		if _, err := read(filepath.Join(dir, "absent")); reasonCode(err) != "config_missing" {
			t.Errorf("%s missing file reason = %q", name, reasonCode(err))
		}
		// a directory at the config path exists but cannot be read as a file
		if _, err := read(dir); reasonCode(err) != "config_unreadable" {
			t.Errorf("%s directory reason = %q", name, reasonCode(err))
		}
	}
}

func TestReadArtifactFactsInterpretsHostEventShapes(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  artifactFacts
	}{
		{"codex wire revision and catalog", []string{
			`{"direction":"host->client","raw":"{\"id\":1,\"result\":{\"protocolVersion\":\"2025-06-18\",\"data\":[{\"tools\":{\"docs_index\":{}}}]}}"}`,
		}, artifactFacts{catalogListsDocs: true, revisions: map[string]bool{"2025-06-18": true}}},
		{"codex wire tool call success", []string{
			`{"direction":"client->host","method":"initialize"}`,
			`{"direction":"client->host","method":"mcpServer/tool/call"}`,
			`{"direction":"host->client","raw":"{\"id\":2,\"result\":{\"content\":[{\"text\":\"not json\"},{\"text\":\"{\\\"docs\\\":[]}\"}]}}"}`,
		}, artifactFacts{docsSucceeded: true, revisions: map[string]bool{}}},
		{"codex wire response for non call id is ignored", []string{
			`{"direction":"client->host","method":"mcpServer/tool/call"}`,
			`{"direction":"host->client","raw":"{\"id\":7,\"result\":{\"content\":[{\"text\":\"{\\\"docs\\\":[]}\"}]}}"}`,
		}, artifactFacts{revisions: map[string]bool{}}},
		{"codex wire tool call error", []string{
			`{"direction":"client->host","method":"mcpServer/tool/call"}`,
			`{"direction":"host->client","raw":"{\"id\":1,\"error\":{\"code\":-1}}"}`,
		}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"codex wire error on non call is not a tool failure", []string{
			`{"direction":"client->host","method":"initialize"}`,
			`{"direction":"host->client","raw":"{\"id\":1,\"error\":{\"code\":-1}}"}`,
		}, artifactFacts{revisions: map[string]bool{}}},
		{"codex wire isError result", []string{
			`{"direction":"client->host","method":"mcpServer/tool/call"}`,
			`{"direction":"host->client","raw":"{\"id\":1,\"result\":{\"isError\":true,\"content\":[{\"text\":\"{\\\"docs\\\":[]}\"}]}}"}`,
		}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"codex wire payload ok false", []string{
			`{"direction":"client->host","method":"mcpServer/tool/call"}`,
			`{"direction":"host->client","raw":"{\"id\":1,\"result\":{\"content\":[{\"text\":\"{\\\"ok\\\":false}\"}]}}"}`,
		}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"codex wire unparsable raw and result-less message", []string{
			`{"direction":"host->client","raw":"garbage"}`,
			`{"direction":"host->client","raw":"{\"id\":1}"}`,
			`{"direction":"other"}`,
		}, artifactFacts{revisions: map[string]bool{}}},
		{"claude init lists namespaced tool and call succeeds", []string{
			`{"type":"system","subtype":"init","tools":["Bash","mcp__issueops__docs_index"]}`,
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"mcp__issueops__docs_index","id":"u1"},{"type":"tool_use","name":"Bash","id":"u2"},{"type":"text"}]}}`,
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"u2"},{"type":"tool_result","tool_use_id":"u1"}]}}`,
		}, artifactFacts{catalogListsDocs: true, docsSucceeded: true, revisions: map[string]bool{}}},
		{"claude tool result error", []string{
			`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"docs_index","id":"u1"}]}}`,
			`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"u1","is_error":true}]}}`,
		}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"claude non init system and unrelated type", []string{
			`{"type":"system","subtype":"other","tools":["docs_index"]}`,
			`{"type":"result"}`,
		}, artifactFacts{revisions: map[string]bool{}}},
		{"codex steps inventory and tool outcomes", []string{
			`{"step":"inventory","servers":[{"tools":["a","docs_index"]},"junk"]}`,
			`{"step":"tool","tool":"other","ok":false}`,
			`{"step":"tool","tool":"docs_index","ok":true}`,
		}, artifactFacts{catalogListsDocs: true, docsSucceeded: true, revisions: map[string]bool{}}},
		{"codex steps tool isError", []string{
			`{"step":"tool","tool":"docs_index","isError":true}`,
			`{"step":"unknown"}`,
		}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"codex steps tool without verdict proves nothing", []string{
			`{"step":"tool","tool":"docs_index"}`,
		}, artifactFacts{revisions: map[string]bool{}}},
		{"omo success with revisions", []string{
			`{"ok":true,"docs":3,"tools":["docs_index"],"revisions":["2025-06-18",""],"mcp_protocol_version_headers":["2025-03-26"]}`,
		}, artifactFacts{catalogListsDocs: true, docsSucceeded: true, revisions: map[string]bool{"2025-06-18": true, "2025-03-26": true}}},
		{"omo has_docs_index flag", []string{`{"ok":true,"has_docs_index":true}`}, artifactFacts{catalogListsDocs: true, revisions: map[string]bool{}}},
		{"omo failure with docs", []string{`{"ok":false,"docs":1,"revisions":["2025-06-18"]}`}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"omo failure without docs proves nothing", []string{`{"ok":false}`}, artifactFacts{revisions: map[string]bool{}}},
		{"omo is_error and isError spellings", []string{`{"ok":true,"docs":1,"is_error":true}`, `{"ok":true,"docs":1,"isError":true}`}, artifactFacts{docsFailed: true, revisions: map[string]bool{}}},
		{"omo line without ok is ignored", []string{`{"docs":1}`}, artifactFacts{revisions: map[string]bool{}}},
		{"non json lines are skipped", []string{``, `plain text docs_index`, `{broken`, `[1]`}, artifactFacts{revisions: map[string]bool{}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "artifact.jsonl")
			if err := os.WriteFile(path, []byte(strings.Join(tc.lines, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			got, ok := readArtifactFacts(path)
			if !ok || !reflect.DeepEqual(*got, tc.want) {
				t.Fatalf("facts = %+v ok=%v, want %+v", got, ok, tc.want)
			}
		})
	}
	if facts, ok := readArtifactFacts(filepath.Join(t.TempDir(), "absent.jsonl")); ok || facts != nil {
		t.Fatalf("missing artifact must not yield facts: %+v %v", facts, ok)
	}
}

func TestListSkillsOnlyIndexesDirectoriesAndReadsFrontmatterDescription(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(root, "skills", rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("zeta/SKILL.md", "---\nname: zeta\ndescription: 'Zeta skill'\n---\n")
	write("zeta/agents/openai.yaml", "x")
	write("alpha/SKILL.md", "no description here\n")
	write("stray.txt", "file, not a skill")

	got := ListSkills(root, "zeta")

	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zeta" {
		t.Fatalf("skills must be directories sorted by name: %+v", got)
	}
	if got[0].Description != "" || !got[0].HasSkillMD || got[0].HasOpenAI {
		t.Fatalf("alpha = %+v", got[0])
	}
	if got[1].Description != "Zeta skill" || !got[1].HasSkillMD || !got[1].HasOpenAI {
		t.Fatalf("zeta = %+v", got[1])
	}
	if ListSkills(t.TempDir(), "x") != nil {
		t.Fatal("missing skills dir must yield nil")
	}
}
