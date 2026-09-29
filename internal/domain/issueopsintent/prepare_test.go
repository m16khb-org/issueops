package issueopsintent

import (
	"strings"
	"testing"
)

func TestPrepareIntentRules(t *testing.T) {
	valid := IntentDraft{RawRequest: " fix login ", InterpretedIntent: " recover an expired session ", SuccessCriteria: []string{" ok ", "ok", ""}}
	cases := []struct {
		name   string
		change func(*IntentDraft)
		want   string
	}{
		{"empty raw", func(r *IntentDraft) { r.RawRequest = " " }, "raw_request is required"},
		{"empty interpretation", func(r *IntentDraft) { r.InterpretedIntent = " " }, "interpreted_intent is required"},
		{"copied request", func(r *IntentDraft) { r.InterpretedIntent = r.RawRequest }, "must differ"},
		{"cosmetic rewrite", func(r *IntentDraft) {
			r.RawRequest = "please fix broken login session refresh"
			r.InterpretedIntent = "fix broken login session refresh please"
		}, "materially differ"},
		{"no criteria", func(r *IntentDraft) { r.SuccessCriteria = nil }, "success_criteria is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := valid
			tc.change(&req)
			_, err := PrepareIntent(req)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v", err)
			}
		})
	}
	intent, err := PrepareIntent(valid)
	if err != nil || intent.RawRequest != "fix login" || intent.InterpretedIntent != "recover an expired session" || len(intent.SuccessCriteria) != 1 {
		t.Fatalf("intent=%+v err=%v", intent, err)
	}
}
