package issueops

import "testing"

func TestValidateOrcaRequestID(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{"lowercase uuid", "123e4567-e89b-12d3-a456-426614174000", false},
		{"uppercase uuid", "123E4567-E89B-12D3-A456-426614174000", false},
		{"uuid with surrounding whitespace", "  123e4567-e89b-12d3-a456-426614174000\n", false},
		{"empty", "", true},
		{"whitespace only", "   ", true},
		{"missing hyphens", "123e4567e89b12d3a456426614174000", true},
		{"short last group", "123e4567-e89b-12d3-a456-42661417400", true},
		{"non-hex digit", "123e4567-e89b-12d3-a456-42661417400g", true},
		{"trailing text", "123e4567-e89b-12d3-a456-426614174000-x", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateOrcaRequestID(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateOrcaRequestID(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
		})
	}
}

func TestValidateOrcaRetryRequestIDAllowsBlankButNotMalformed(t *testing.T) {
	for _, blank := range []string{"", "  \t"} {
		if err := ValidateOrcaRetryRequestID(blank); err != nil {
			t.Fatalf("blank retry id %q: %v", blank, err)
		}
	}
	if err := ValidateOrcaRetryRequestID("123e4567-e89b-12d3-a456-426614174000"); err != nil {
		t.Fatalf("valid retry id: %v", err)
	}
	if err := ValidateOrcaRetryRequestID("not-a-uuid"); err == nil {
		t.Fatal("malformed non-blank retry id was accepted")
	}
}
