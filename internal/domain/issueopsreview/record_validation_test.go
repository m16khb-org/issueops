package issueopsreview

import "testing"

func TestValidateImplementationReviewRecord(t *testing.T) {
	for _, test := range []struct {
		verdict   string
		findings  int
		evidence  int
		wantError string
	}{
		{"pass", 1, 1, ""},
		{"unknown", 1, 1, "implementation review verdict must be pass|revise|stop"},
		{"pass", 0, 1, "implementation review requires at least one finding and one evidence entry"},
		{"pass", 1, 0, "implementation review requires at least one finding and one evidence entry"},
	} {
		if err := ValidateImplementationReviewRecord(test.verdict, test.findings, test.evidence); errorText(err) != test.wantError {
			t.Fatalf("ValidateImplementationReviewRecord(%q, %d, %d) = %v", test.verdict, test.findings, test.evidence, err)
		}
	}
}

func TestValidateProjectDocsReviewRecord(t *testing.T) {
	for _, test := range []struct {
		verdict   string
		docs      int
		evidence  int
		reviewed  int
		wantError string
	}{
		{"updated", 1, 1, 0, ""},
		{"no-change", 0, 1, 1, ""},
		{"unknown", 0, 0, 0, "project docs review verdict must be updated|no-change"},
		{"updated", 1, 0, 0, "project docs review requires at least one evidence entry"},
		{"updated", 0, 1, 0, "project docs review verdict updated requires at least one --doc path"},
		{"no-change", 1, 1, 1, "project docs review verdict no-change must not list updated docs"},
		{"no-change", 0, 1, 0, "project docs review verdict no-change requires at least one --reviewed-doc path that was actually read"},
	} {
		if err := ValidateProjectDocsReviewRecord(test.verdict, test.docs, test.evidence, test.reviewed); errorText(err) != test.wantError {
			t.Fatalf("ValidateProjectDocsReviewRecord(%q, %d, %d, %d) = %v", test.verdict, test.docs, test.evidence, test.reviewed, err)
		}
	}
}

func TestValidateSchemaEvidenceRecord(t *testing.T) {
	for _, test := range []struct {
		waive        bool
		rationale    string
		measurements int
		sources      int
		wantError    string
	}{
		{false, "", 1, 1, ""},
		{true, "reason", 0, 0, ""},
		{true, "", 0, 0, "schema evidence waiver requires --waiver-rationale"},
		{false, "", 0, 1, "schema evidence requires at least one --measurement or an explicit --waive"},
		{false, "", 1, 0, "schema evidence requires at least one --source naming where the measurement was observed"},
	} {
		if err := ValidateSchemaEvidenceRecord(test.waive, test.rationale, test.measurements, test.sources); errorText(err) != test.wantError {
			t.Fatalf("ValidateSchemaEvidenceRecord(%t, %q, %d, %d) = %v", test.waive, test.rationale, test.measurements, test.sources, err)
		}
	}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
