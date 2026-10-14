package gates

import (
	"strings"
	"testing"
)

func TestLedgerFormatWarningsNameAttributesTheParserCannotRead(t *testing.T) {
	text := `# Gates: x

- [x] G1: lists unmet gates | CHECK: go test ./internal/domain/gates -count=1 | EXPECT: ok
  - EVIDENCE: ok  	issueops/internal/domain/gates	0.31s
- [x] G2: prints a FAILED line
  CHECK: go test ./cmd/issueops/gatescli -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/gatescli	0.2s
- [x] G3: unindented list attribute
- EVIDENCE: ok
`
	warnings := LedgerFormatWarnings("gates.md", Parse(text))
	if len(warnings) != 3 {
		t.Fatalf("warnings = %q, want G1's inline CHECK and list-item EVIDENCE and G3's unindented list-item EVIDENCE; well-formed G2 gets none", warnings)
	}
	if !strings.HasPrefix(warnings[0], "gates.md G1: CHECK/EXPECT are on the checkbox line") ||
		!strings.HasPrefix(warnings[1], "gates.md G1: EVIDENCE is written as a list item") ||
		!strings.HasPrefix(warnings[2], "gates.md G3: EVIDENCE is written as a list item") {
		t.Fatalf("warnings = %q", warnings)
	}
}
