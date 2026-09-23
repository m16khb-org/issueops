package issueopscli

import (
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	domaincli "issueops/internal/domain/cli"
	"issueops/internal/domain/commandparse"
)

// 같은 명령 문법이 세 곳에 있다: canonical usage 카탈로그, handler마다 만드는
// FlagSet, 생성 명령과 owner mutation을 exact로 파싱하는 commandparse spec.
// 기존 parity 테스트는 "카탈로그가 광고한 flag ⊆ spec" 한 방향만 봤고, 카탈로그가
// 축약본이라는 전제로 역방향을 보지 않았다. 카탈로그가 전체 --help를 렌더하게 된
// 뒤에는 그 전제가 맞지 않는다. 이 테스트는 카탈로그의 모든 명령에 대해
// 카탈로그 = FlagSet, 그리고 spec이 있으면 spec = FlagSet을 요구한다.
func TestIssueOpsCommandGrammarAgreesAcrossCatalogFlagSetAndSpec(t *testing.T) {
	// 같은 변수에 묶인 호환 alias는 카탈로그에 적지 않는다.
	undocumentedAliases := map[string][]string{
		"remote verify-artifact": {"--labels", "--assignees"},
	}
	var mismatches []string
	for _, line := range domaincli.IssueOpsUsageLines() {
		key := domaincli.IssueOpsUsageKey(line)
		if key == "" {
			continue
		}
		flagSet := flagSetFlagsForCommand(t, strings.Fields(key))
		if len(flagSet) == 0 {
			mismatches = append(mismatches, key+": FlagSet did not report any flag")
			continue
		}
		for _, alias := range undocumentedAliases[key] {
			delete(flagSet, alias)
		}
		catalog := catalogAdvertisedFlags(line)
		if missing := flagDifference(flagSet, catalog); len(missing) > 0 {
			mismatches = append(mismatches, key+": FlagSet accepts flags the catalog does not document "+strings.Join(missing, " "))
		}
		if extra := flagDifference(catalog, flagSet); len(extra) > 0 {
			mismatches = append(mismatches, key+": catalog documents flags the FlagSet rejects "+strings.Join(extra, " "))
		}
		values, booleans, _, ok := commandparse.IssueOpsCommandSpec(key)
		if !ok {
			continue
		}
		spec := map[string]bool{}
		for name := range values {
			spec[name] = true
		}
		for name := range booleans {
			spec[name] = true
		}
		for _, alias := range undocumentedAliases[key] {
			delete(spec, alias)
		}
		if extra := flagDifference(spec, flagSet); len(extra) > 0 {
			mismatches = append(mismatches, key+": exact-parse spec accepts flags the FlagSet rejects "+strings.Join(extra, " "))
		}
		if missing := flagDifference(flagSet, spec); len(missing) > 0 {
			mismatches = append(mismatches, key+": exact-parse spec rejects flags the FlagSet accepts "+strings.Join(missing, " "))
		}
	}
	if len(mismatches) > 0 {
		t.Fatalf("issueops command grammar drifted:\n%s", strings.Join(mismatches, "\n"))
	}
}

var flagSetDefaultsLine = regexp.MustCompile(`(?m)^\s+-{1,2}([a-z0-9][a-z0-9-]*)`)

// flagSetFlagsForCommand는 알 수 없는 flag로 handler를 호출해 FlagSet이 출력하는
// 기본값 목록을 읽는다. -h를 먼저 가로채 고유 usage만 출력하는 handler도 알 수 없는
// flag에서는 FlagSet 파싱 오류와 함께 전체 flag를 출력한다. 파싱이 실패하므로
// handler는 state나 원격에 닿지 않는다.
func flagSetFlagsForCommand(t *testing.T, path []string) map[string]bool {
	t.Helper()
	oldStdout, oldStderr := os.Stdout, os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = writer, writer
	output := make(chan string)
	go func() {
		data, _ := io.ReadAll(reader)
		output <- string(data)
	}()
	_ = runIssueOps(append(append([]string{}, path...), "--issueops-grammar-probe"))
	_ = writer.Close()
	os.Stdout, os.Stderr = oldStdout, oldStderr
	flags := map[string]bool{}
	for _, match := range flagSetDefaultsLine.FindAllStringSubmatch(<-output, -1) {
		flags["--"+match[1]] = true
	}
	return flags
}

func catalogAdvertisedFlags(line string) map[string]bool {
	recordActor := []string{"--host", "--session-id", "--agent-id", "--cwd"}
	fullActor := append(append([]string{}, recordActor...), "--session-pid", "--session-started-at", "--session-executable")
	flags := map[string]bool{}
	for _, field := range strings.Fields(line) {
		switch strings.Trim(field, "[]()") {
		case "RECORD_ACTOR_FLAGS":
			for _, name := range recordActor {
				flags[name] = true
			}
			continue
		case "ACTOR_FLAGS":
			for _, name := range fullActor {
				flags[name] = true
			}
			continue
		}
		for _, part := range strings.Split(strings.Trim(field, "[](),.:"), "|") {
			part = strings.TrimSuffix(strings.Trim(part, "[](),.:"), "...")
			if strings.HasPrefix(part, "--") {
				flags[part] = true
			}
		}
	}
	return flags
}

func flagDifference(left, right map[string]bool) []string {
	var out []string
	for name := range left {
		if !right[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
