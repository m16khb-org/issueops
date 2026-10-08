package nativeactivation

import (
	"reflect"
	"strings"
	"testing"

	activationcontract "issueops/internal/contract/nativeactivation"
)

func TestReadbackOrderRequiresExactlyNineFirstPartySurfaces(t *testing.T) {
	evidence := fixtureEvidence()
	order, err := ReadbackOrder(strings.Repeat("a", 64), evidence)
	if err != nil || !reflect.DeepEqual(order, []int{8, 1, 5, 4, 0, 6, 2, 7, 3}) {
		t.Fatalf("readback order=%v err=%v", order, err)
	}
	if len(evidence) != 9 || evidence[0].Host != "codex" || evidence[0].Surface != "mcp" {
		t.Fatal("validation mutated input evidence")
	}
	missing := append([]activationcontract.Evidence(nil), evidence[:8]...)
	if _, err := ReadbackOrder(strings.Repeat("a", 64), missing); err == nil {
		t.Fatal("missing agy readback accepted")
	}
	withoutOmpHooks := append(append([]activationcontract.Evidence(nil), evidence[:7]...), evidence[8])
	if _, err := ReadbackOrder(strings.Repeat("a", 64), withoutOmpHooks); err == nil {
		t.Fatal("missing omp hooks readback accepted")
	}
	duplicate := append([]activationcontract.Evidence(nil), evidence...)
	duplicate[8] = duplicate[0]
	if _, err := ReadbackOrder(strings.Repeat("a", 64), duplicate); err == nil {
		t.Fatal("duplicate surface accepted")
	}
}

func TestReadbackOrderRejectsNoncanonicalEvidence(t *testing.T) {
	evidence := fixtureEvidence()
	evidence[0].Host = " codex"
	if _, err := ReadbackOrder(strings.Repeat("a", 64), evidence); err == nil {
		t.Fatal("noncanonical host accepted")
	}
	evidence = fixtureEvidence()
	evidence[0].SHA256 = "bad"
	if _, err := ReadbackOrder(strings.Repeat("a", 64), evidence); err == nil {
		t.Fatal("invalid digest accepted")
	}
}

func fixtureEvidence() []activationcontract.Evidence {
	items := [][2]string{{"codex", "mcp"}, {"claude", "hooks"}, {"omo", "mcp"}, {"omp", "mcp"}, {"codex", "hooks"}, {"claude", "mcp"}, {"omo", "hooks"}, {"omp", "hooks"}, {"agy", "mcp"}}
	out := make([]activationcontract.Evidence, 0, len(items))
	for _, item := range items {
		out = append(out, activationcontract.Evidence{
			Host: item[0], Surface: item[1], Path: "/" + item[0] + "/" + item[1],
			SemanticSHA256: strings.Repeat("b", 64), SHA256: strings.Repeat("c", 64),
		})
	}
	return out
}
