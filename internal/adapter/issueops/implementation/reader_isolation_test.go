package implementation

import (
	model "issueops/internal/contract/issueops"
	"reflect"
	"testing"
)

func TestChangeReadersKeepTheirGitObservers(t *testing.T) {
	record := model.IssueOpsRecord{Repo: t.TempDir()}
	capture := func(path string) func(model.IssueOpsRecord) []string {
		reader := Reader{}
		reader.GitCmd = func(string, ...string) (int, string, string) { return 0, "true", "" }
		reader.GitCmdRaw = func(string, ...string) (int, string, string) { return 0, "?? " + path + "\n", "" }
		return reader.ChangedPaths
	}
	first, second := capture("first.go"), capture("second.go")
	for _, tc := range []struct {
		read func(model.IssueOpsRecord) []string
		want string
	}{{first, "first.go"}, {second, "second.go"}, {first, "first.go"}} {
		if got := tc.read(record); !reflect.DeepEqual(got, []string{tc.want}) {
			t.Fatalf("reader changed its Git observer: got=%v want=%s", got, tc.want)
		}
	}
}
