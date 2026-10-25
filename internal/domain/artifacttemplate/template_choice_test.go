package artifacttemplate

import (
	"strings"
	"testing"
)

// Probe: an unknown kind or template rendered another template's skeleton.
func TestChoiceErrorNamesTheValidValues(t *testing.T) {
	for _, tc := range []struct{ kind, template, want string }{
		{"bogus", "", `unsupported artifact kind "bogus": want issue|child|pr`},
		{"pr", "nope", `unsupported template "nope" for kind pr: want pull_request`},
		{"issue", "child_task", "want bug|feature|proposal|implementation_task"},
	} {
		if err := ChoiceError(tc.kind, tc.template); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s/%s: err = %v", tc.kind, tc.template, err)
		}
	}
	for _, ok := range [][2]string{{"", ""}, {"PR", ""}, {"issue", "bug"}, {"child", ""}, {"child", "child_task"}} {
		if err := ChoiceError(ok[0], ok[1]); err != nil {
			t.Fatalf("%v: %v", ok, err)
		}
	}
}
