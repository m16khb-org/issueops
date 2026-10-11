package issueopsbenchmark

import (
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsbenchmark"
)

const databaseDesignEvidence = `schema: row count 1M
explain: query plan uses seq scan
index: idx_a
write penalty noted
normalization: 3nf holds`

func TestPioneerSkillEvidenceComplete(t *testing.T) {
	cases := []struct {
		name     string
		target   string
		evidence string
		want     bool
	}{
		{"complete", "database-design", databaseDesignEvidence, true},
		{"target is case/space insensitive", "  Database-Design ", databaseDesignEvidence, true},
		{"unknown target", "no-such-skill", databaseDesignEvidence, false},
		{"empty target", "", databaseDesignEvidence, false},
		{"empty evidence", "database-design", "", false},
		{"structured clause needs colon", "database-design", strings.Replace(databaseDesignEvidence, "index: idx_a", "index idx_a", 1), false},
		{"marker with colon on other line does not count", "database-design", strings.Replace(databaseDesignEvidence, "index: idx_a", "idx\nnote: x", 1), false},
		{"unstructured clause satisfied anywhere", "database-design", strings.Replace(databaseDesignEvidence, "write penalty noted", "insert cost: low", 1), true},
		{"unstructured clause missing", "database-design", strings.Replace(databaseDesignEvidence, "write penalty noted", "", 1), false},
		{"markers case-insensitive", "database-design", strings.ToUpper(databaseDesignEvidence), true},
		{"algorithm complete", "algorithm-optimization", "hot path: profile\ncomplexity: o(n)\nscaling: n=1000\ninvariant: correctness\nbefore/after: before 5ms", true},
		{"web-research timestamp year marker", "web-research", "source fan-out: 3\nsource index: sources\nclaim verification: verification status ok\naccess boundary: paywall\nfetched 2026-01-01", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := PioneerSkillEvidenceComplete(
				contract.IssueOpsBenchmarkFixture{PioneerSkillTarget: c.target},
				contract.IssueOpsBenchmarkArtifact{PioneerSkillEvidence: c.evidence})
			if got != c.want {
				t.Fatalf("got %v want %v", got, c.want)
			}
		})
	}
}

func TestHasStructuredClause(t *testing.T) {
	cases := []struct {
		text    string
		markers []string
		want    bool
	}{
		{"index: a", []string{"index"}, true},
		{"INDEX: a", []string{"index"}, true},
		{"index a", []string{"index"}, false},
		{"a: b\nindex c", []string{"index"}, false},
		{"x: y", []string{"nope", "y"}, true},
		{"", []string{"a"}, false},
		{"a: b", nil, false},
	}
	for _, c := range cases {
		if got := hasStructuredClause(c.text, c.markers...); got != c.want {
			t.Errorf("%q %v: got %v", c.text, c.markers, got)
		}
	}
}
