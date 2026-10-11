package installutil

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const harnessHookCommand = "'/source/bin/issueops' hook pre-tool-use --host codex"

func harnessGroup(command string) map[string]any {
	return map[string]any{"hooks": []any{map[string]any{"type": "command", "command": command}}}
}

func thirdPartyGroup() map[string]any {
	return map[string]any{"hooks": []any{map[string]any{"type": "command", "command": "/bin/sh /opt/other/hook.sh"}}}
}

func TestVerifyHookConfigActivationAcceptsCoResidentThirdPartyGroupsAndReturnsExpectedDigest(t *testing.T) {
	expected := hookTargetTestConfig(harnessHookCommand)
	actual := map[string]any{"hooks": map[string]any{
		"PreToolUse": []any{thirdPartyGroup(), harnessGroup(harnessHookCommand)},
		"Other":      []any{thirdPartyGroup()},
	}}
	got, err := VerifyHookConfigActivation(actual, expected)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(expected)
	sum := sha256.Sum256(raw)
	if got != hex.EncodeToString(sum[:]) {
		t.Fatalf("digest = %s, want digest of the expected catalog", got)
	}
}

func TestVerifyHookConfigActivationRejections(t *testing.T) {
	expected := hookTargetTestConfig(harnessHookCommand)
	tests := []struct {
		name     string
		actual   map[string]any
		expected map[string]any
		want     string
	}{
		{"empty expected catalog", expected, map[string]any{"hooks": map[string]any{}}, "expected hook catalog is empty"},
		{"expected catalog absent", expected, map[string]any{}, "expected hook catalog is empty"},
		{"installed catalog missing", map[string]any{}, expected, "installed hook catalog is missing"},
		{"installed catalog wrong type", map[string]any{"hooks": []any{}}, expected, "installed hook catalog is missing"},
		{"expected event missing", map[string]any{"hooks": map[string]any{"Other": []any{thirdPartyGroup()}}}, expected, "installed hook event PreToolUse is missing"},
		{"event groups malformed", map[string]any{"hooks": map[string]any{"PreToolUse": "oops"}}, expected, "installed hook event PreToolUse is malformed"},
		{
			"unexpected harness group on foreign event",
			map[string]any{"hooks": map[string]any{
				"PreToolUse": []any{harnessGroup(harnessHookCommand)},
				"Stop":       []any{harnessGroup("'/source/bin/issueops' hook stop --host codex")},
			}},
			expected, "installed hook event Stop contains an unexpected issueops group",
		},
		{
			"duplicate harness group",
			map[string]any{"hooks": map[string]any{"PreToolUse": []any{harnessGroup(harnessHookCommand), harnessGroup(harnessHookCommand)}}},
			expected, "does not contain exactly one canonical issueops group",
		},
		{
			"only third party group on expected event",
			map[string]any{"hooks": map[string]any{"PreToolUse": []any{thirdPartyGroup()}}},
			expected, "does not contain exactly one canonical issueops group",
		},
		{
			"harness group differs from expected content",
			map[string]any{"hooks": map[string]any{"PreToolUse": []any{harnessGroup(harnessHookCommand + " --extra")}}},
			expected, "does not contain exactly one canonical issueops group",
		},
		{
			"expected groups malformed",
			map[string]any{"hooks": map[string]any{"PreToolUse": []any{harnessGroup(harnessHookCommand)}}},
			map[string]any{"hooks": map[string]any{"PreToolUse": "oops"}},
			"does not contain exactly one canonical issueops group",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			digest, err := VerifyHookConfigActivation(tt.actual, tt.expected)
			if err == nil || !strings.Contains(err.Error(), tt.want) || digest != "" {
				t.Fatalf("digest=%q err=%v, want error containing %q", digest, err, tt.want)
			}
		})
	}
}

func TestVerifyHookActivationReadFailures(t *testing.T) {
	dir := t.TempDir()
	expected := hookTargetTestConfig(harnessHookCommand)
	if _, err := VerifyHookActivation(filepath.Join(dir, "missing.json"), expected); !os.IsNotExist(err) {
		t.Fatalf("missing file err = %v", err)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := VerifyHookActivation(bad, expected); err == nil {
		t.Fatal("malformed JSON was accepted")
	}
}

func TestVerifyHookActivationAcceptsInstalledFile(t *testing.T) {
	config := hookTargetTestConfig(harnessHookCommand)
	raw, _ := json.Marshal(config)
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := VerifyHookActivation(path, config)
	if err != nil || len(digest) != 64 {
		t.Fatalf("digest=%q err=%v", digest, err)
	}
}

func TestSemanticSHA256RejectsUnmarshalableValueAndDistinguishesContent(t *testing.T) {
	if _, err := SemanticSHA256(map[string]any{"bad": make(chan int)}); err == nil {
		t.Fatal("unmarshalable value was hashed")
	}
	first, _ := SemanticSHA256(map[string]any{"a": 1, "b": 2})
	same, _ := SemanticSHA256(map[string]any{"b": 2, "a": 1})
	other, _ := SemanticSHA256(map[string]any{"a": 1, "b": 3})
	if first != same || first == other {
		t.Fatalf("semantic hashes first=%s same=%s other=%s", first, same, other)
	}
}

func TestCanonicalJSONEqualComparesDecodedValues(t *testing.T) {
	if !canonicalJSONEqual([]string{"a", "b"}, []any{"a", "b"}) {
		t.Fatal("typed and decoded slices with equal JSON must compare equal")
	}
	if canonicalJSONEqual(map[string]any{"a": 1}, map[string]any{"a": 2}) {
		t.Fatal("different values compared equal")
	}
	if canonicalJSONEqual(make(chan int), make(chan int)) || canonicalJSONEqual(1, make(chan int)) {
		t.Fatal("unmarshalable values must never compare equal")
	}
}

func TestCaptureNativeActivationEvidenceRecordsFileIdentity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	content := []byte(`{"hooks":{}}`)
	if err := os.WriteFile(path, content, 0o640); err != nil {
		t.Fatal(err)
	}
	evidence, err := CaptureNativeActivationEvidence("codex", "hooks", path, "semantic")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	info, _ := os.Stat(path)
	if evidence.SHA256 != hex.EncodeToString(sum[:]) || evidence.Size != int64(len(content)) ||
		evidence.Mode != uint32(info.Mode()) || evidence.Host != "codex" || evidence.Surface != "hooks" ||
		evidence.SemanticSHA256 != "semantic" || evidence.Inode == 0 || evidence.Device == 0 {
		t.Fatalf("evidence = %+v", evidence)
	}
	resolved, _ := filepath.EvalSymlinks(path)
	got, _ := filepath.EvalSymlinks(evidence.Path)
	if got != resolved {
		t.Fatalf("path = %s, want %s", got, resolved)
	}
}

func TestCaptureNativeActivationEvidenceRejectsNonPhysicalFiles(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "real.json")
	if err := os.WriteFile(regular, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(dir, "link.json")
	if err := os.Symlink(regular, symlink); err != nil {
		t.Fatal(err)
	}
	hardlink := filepath.Join(dir, "hard.json")
	if err := os.Link(regular, hardlink); err != nil {
		t.Fatal(err)
	}
	subdir := filepath.Join(dir, "sub")
	if err := os.Mkdir(subdir, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, path := range map[string]string{"symlink": symlink, "hardlink": hardlink, "directory": subdir} {
		if _, err := CaptureNativeActivationEvidence("codex", "hooks", path, ""); err == nil ||
			!strings.Contains(err.Error(), "physical single-link regular file") {
			t.Fatalf("%s err = %v", name, err)
		}
	}
	if _, err := CaptureNativeActivationEvidence("codex", "hooks", filepath.Join(dir, "absent"), ""); !os.IsNotExist(err) {
		t.Fatalf("missing err = %v", err)
	}
}

func TestValidateHookConfigForMerge(t *testing.T) {
	events := []string{"PreToolUse", "Stop"}
	tests := []struct {
		name    string
		config  map[string]any
		wantErr string
	}{
		{"no catalog", map[string]any{"other": 1}, ""},
		{"valid catalog", map[string]any{"hooks": map[string]any{"PreToolUse": []any{}, "Unknown": "free-form"}}, ""},
		{"catalog wrong type", map[string]any{"hooks": []any{}}, "installed hook catalog is malformed"},
		{"known event wrong type", map[string]any{"hooks": map[string]any{"Stop": map[string]any{}}}, "installed hook event Stop is malformed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateHookConfigForMerge(tt.config, events)
			if tt.wantErr == "" && err != nil || tt.wantErr != "" && (err == nil || err.Error() != tt.wantErr) {
				t.Fatalf("err = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestHookGroupContainsCommandMatchesPrefixOnlyOnWellFormedGroups(t *testing.T) {
	group := map[string]any{"hooks": []any{
		"not-a-map",
		map[string]any{"command": 7},
		map[string]any{"command": "/usr/bin/tool run"},
	}}
	if !HookGroupContainsCommand(group, "/usr/bin/tool") {
		t.Fatal("prefix match was missed")
	}
	if HookGroupContainsCommand(group, "tool") {
		t.Fatal("non-prefix substring matched")
	}
	for _, malformed := range []any{nil, "x", map[string]any{"hooks": "x"}, map[string]any{}} {
		if HookGroupContainsCommand(malformed, "") {
			t.Fatalf("malformed group %#v matched", malformed)
		}
		if HookGroupContainsAgentHarness(malformed) {
			t.Fatalf("malformed group %#v classified as managed", malformed)
		}
	}
}

func TestRemoveJSONMapEntry(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	config, removed, err := RemoveJSONMapEntry(write("a.json", `{"theme":"dark","mcpServers":{"issueops":{"c":1},"other":{"c":2}}}`), "mcpServers", "issueops")
	if err != nil || !removed {
		t.Fatalf("removed=%v err=%v", removed, err)
	}
	servers := config["mcpServers"].(map[string]any)
	if _, still := servers["issueops"]; still || servers["other"] == nil || config["theme"] != "dark" {
		t.Fatalf("config = %#v", config)
	}
	for name, path := range map[string]string{
		"missing file":  filepath.Join(dir, "absent.json"),
		"blank file":    write("blank.json", " \n"),
		"entry absent":  write("noentry.json", `{"mcpServers":{"other":{}}}`),
		"parent absent": write("noparent.json", `{"x":1}`),
	} {
		config, removed, err := RemoveJSONMapEntry(path, "mcpServers", "issueops")
		if config != nil || removed || err != nil {
			t.Fatalf("%s: config=%v removed=%v err=%v", name, config, removed, err)
		}
	}
	for name, content := range map[string]string{"malformed": "{", "null": "null"} {
		if _, removed, err := RemoveJSONMapEntry(write(name+".json", content), "mcpServers", "issueops"); err == nil || removed {
			t.Fatalf("%s: removed=%v err=%v", name, removed, err)
		}
	}
	if _, _, err := RemoveJSONMapEntry(dir, "mcpServers", "issueops"); err == nil {
		t.Fatal("directory read error was swallowed")
	}
}
