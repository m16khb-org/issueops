package trace

import "testing"

func TestDecodeIssueOpsHasSummaryReflectsSummaryObject(t *testing.T) {
	withSummary := Source{}.Decode([]byte(`{"summary":{"failed_steps":1}}`), "")
	if withSummary.Document == nil || !withSummary.Document.HasSummary {
		t.Fatalf("summary object must set HasSummary: %+v", withSummary.Document)
	}
	flat := Source{}.Decode([]byte(`{"event":"step_end","step":"x","ok":true}`), "")
	if flat.Document == nil || flat.Document.HasSummary {
		t.Fatalf("document without summary object must not set HasSummary: %+v", flat.Document)
	}
}
