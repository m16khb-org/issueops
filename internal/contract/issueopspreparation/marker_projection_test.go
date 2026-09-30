package issueopspreparation

import "testing"

func TestOrcaIntentMarkerRoundTripsProviderAndPurposeIdentity(t *testing.T) {
	tests := []struct {
		name     string
		identity MarkerIdentity
		want     string
	}{
		{
			name: "github prepare",
			identity: MarkerIdentity{
				Purpose: PurposePrepare, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 1, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "github", Issue: 69,
			},
			want: "issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69",
		},
		{
			name: "gitlab resume",
			identity: MarkerIdentity{
				Purpose: PurposeResume, LifecycleID: "io-aaaaaaaaaaaa",
				Generation: 2, OperationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Provider: "gitlab", Issue: 2646,
			},
			want: "issueops-v1 resume lifecycle=io-aaaaaaaaaaaa generation=2 operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab issue=2646",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := (IntentCodec{}).RenderMarker(test.identity)
			if err != nil || got != test.want {
				t.Fatalf("render = %q err=%v, want %q", got, err, test.want)
			}
			parsed, err := (IntentCodec{}).ParseMarker(got)
			if err != nil || parsed != test.identity {
				t.Fatalf("parse = %#v err=%v, want %#v", parsed, err, test.identity)
			}
		})
	}
}

func TestOrcaIntentMarkerRejectsPartialDuplicateAndUnknownIdentity(t *testing.T) {
	for _, marker := range []string{
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=gitlab",
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb issue=69",
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github provider=gitlab issue=69",
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=0",
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=bitbucket issue=69",
		"issueops-v1 lifecycle=io-aaaaaaaaaaaa operation=bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb provider=github issue=69 extra=value",
	} {
		if _, err := (IntentCodec{}).ParseMarker(marker); err == nil {
			t.Fatalf("invalid marker was accepted: %q", marker)
		}
	}
}

func TestOrcaIntentContractErrorExposesStableCodeWithoutDuplicatingDetail(t *testing.T) {
	err := &IntentError{Code: "intent_marker_invalid", Detail: "duplicate provider"}
	if got := err.Error(); got != "intent_marker_invalid: duplicate provider" {
		t.Fatalf("error = %q", got)
	}
	fields := err.IssueOpsErrorFields()
	if fields["code"] != "intent_marker_invalid" || len(fields) != 1 {
		t.Fatalf("error fields = %#v", fields)
	}
}
