package issueopsintent

import (
	"strings"
	"testing"
)

func TestBuildPlanPrepItemRequiresExactlyOneEvidencePath(t *testing.T) {
	for _, test := range []struct {
		name                  string
		evidence              []string
		waive                 string
		wantStatus, wantError string
	}{
		{name: "evidence", evidence: []string{" source ", "source"}, wantStatus: "evidence"},
		{name: "waiver", waive: " internal only ", wantStatus: "waived"},
		{name: "both", evidence: []string{"source"}, waive: "reason", wantError: "mutually exclusive"},
		{name: "neither", wantError: "provide evidence or a waive reason"},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, err := BuildPlanPrepItem("decisions", test.evidence, test.waive)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil || item.Status != test.wantStatus {
				t.Fatalf("item = %+v %v", item, err)
			}
		})
	}
}
