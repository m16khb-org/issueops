package issueopsmodeswitch

import (
	"reflect"
	"testing"
)

func TestComparisonRefsKeepsRemoteThenPreparedBase(t *testing.T) {
	if got := ComparisonRefs("branch", " base "); !reflect.DeepEqual(got, []string{"refs/remotes/origin/branch", "base"}) {
		t.Fatal(got)
	}
}
