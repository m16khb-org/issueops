package issueopslease

import "testing"

func TestValidateResumeBeginAuthority(t *testing.T) {
	for _, test := range []struct {
		name string
		facts ResumeBeginAuthority
		allow bool
	}{
		{name: "same authority", facts: ResumeBeginAuthority{LeaseSame: true, BindingSame: true}, allow: true},
		{name: "pending intent", facts: ResumeBeginAuthority{Pending: true, LeaseSame: true, BindingSame: true}},
		{name: "lease changed", facts: ResumeBeginAuthority{BindingSame: true}},
		{name: "binding changed", facts: ResumeBeginAuthority{LeaseSame: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateResumeBeginAuthority(test.facts)
			if (err == nil) != test.allow {
				t.Fatalf("error=%v allow=%v", err, test.allow)
			}
		})
	}
}
