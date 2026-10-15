package provider

import (
	"strings"
	"testing"
)

// Resolve는 provider 이름 디스패치다. 지원 목록 에러가 정확한 이름을
// 안내하는지 잠근다.
func TestResolveDispatch(t *testing.T) {
	for name := range map[string]bool{"github": true, "gitlab": true} {
		resolved, err := Resolve(name)
		if err != nil || resolved == nil {
			t.Fatalf("Resolve(%q) = %v, %v", name, resolved, err)
		}
	}
	if _, err := Resolve("bitbucket"); err == nil || !strings.Contains(err.Error(), "supported: github, gitlab") {
		t.Fatalf("unknown provider error must list supported providers: %v", err)
	}
}

// 병합 능력은 provider 이름에 따라 위임되고, 알 수 없는 이름은 Resolve 오류를
// 그대로 돌려줘야 한다.
func TestResolvePullRequestMerger(t *testing.T) {
	if _, err := ResolvePullRequestMerger("bitbucket"); err == nil || !strings.Contains(err.Error(), "unknown provider") {
		t.Fatalf("unknown provider must propagate Resolve error: %v", err)
	}
	for _, name := range []string{"github", "gitlab"} {
		merger, err := ResolvePullRequestMerger(name)
		if err != nil {
			t.Fatalf("%s must expose merge capability: %v", name, err)
		}
		if merger == nil {
			t.Fatalf("%s: nil merger without error", name)
		}
	}
}
