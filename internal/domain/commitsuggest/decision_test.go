package commitsuggest

import "testing"

func TestSuggestionRequiresNonWhitespaceDiff(t *testing.T) {
	for _, diff := range []string{"", " \n\t"} {
		if NeedsSuggestion(diff) {
			t.Fatalf("empty diff %q requires suggestion", diff)
		}
	}
	if !NeedsSuggestion(" diff ") {
		t.Fatal("nonempty diff ignored")
	}
}
