package issueopspublication

import "testing"

func TestPendingPublicationEligibility(t *testing.T) {
	for _, tc := range []struct {
		name              string
		prepared, pending bool
		kind              string
		valid             bool
	}{
		{"unprepared", false, false, "", false},
		{"no intent", true, false, "", false},
		{"other intent", true, true, "orca", false},
		{"publication", true, true, "remote_pr_create", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidatePublicationPending(tc.prepared, tc.pending, tc.kind)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || err.Error() != "remote publication intent is not pending" {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
