package issueops

import (
	"strings"
	"testing"
)

func TestOwnerSnapshotValidation(t *testing.T) {
	body := "## Acceptance\n- AC-01: works\n## Verification\n```sh\ngo test ./...\n```\n"
	for _, tc := range []struct {
		name, linked, observed, body string
		limit                        int
		want                         string
	}{
		{"valid", "url", " url ", body, len(body), ""},
		{"wrong identity", "url", "other", body, len(body), "identity or bounded body"},
		{"empty body", "url", "url", " \n", 100, "identity or bounded body"},
		{"oversized body", "url", "url", body, len(body) - 1, "identity or bounded body"},
		{"missing acceptance", "url", "url", "## Verification\n```\ngo test ./...\n```", 100, "acceptance IDs"},
		{"missing command block", "url", "url", "AC-01: works", 100, "exact verification command block"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acceptance, verification, err := ValidateOwnerSnapshot(tc.linked, tc.observed, tc.body, tc.limit)
			if tc.want != "" {
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("expected %q, got %v", tc.want, err)
				}
				return
			}
			if err != nil || strings.Join(acceptance, ",") != "AC-01" || strings.Join(verification, ",") != "go test ./..." {
				t.Fatalf("snapshot validation: %v, %v, %v", acceptance, verification, err)
			}
		})
	}
}
