package guard

import (
	"reflect"
	"testing"

	guardcontract "issueops/internal/contract/guard"
)

type sourceStub struct {
	files []string
	reads []string
}

func (source *sourceStub) ResolveRoot(string) string { return "/repo" }
func (source *sourceStub) TargetFiles(string, guardcontract.GuardCheckRequest) []string {
	return source.files
}
func (source *sourceStub) ExistingSymbols(string, []string) map[string][]string { return nil }
func (source *sourceStub) ReadFile(_ string, rel string, _ bool) (string, bool) {
	source.reads = append(source.reads, rel)
	return "package app\n", true
}

func TestServiceSkipsSecretContentObservationAndUsesDomainAnalysis(t *testing.T) {
	source := &sourceStub{files: []string{".env", "cmd/app/main.go"}}
	result := (Service{Source: source}).Check(guardcontract.GuardCheckRequest{RepoRoot: "/repo", Files: source.files})
	if result.OK || result.RepoRoot != "/repo" || result.Mode != "files" || result.Summary.Block != 1 || result.Summary.Warn < 1 {
		t.Fatalf("guard result = %+v", result)
	}
	if !reflect.DeepEqual(source.reads, []string{"cmd/app/main.go"}) {
		t.Fatalf("content reads = %v", source.reads)
	}
}
