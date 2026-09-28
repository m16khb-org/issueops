package issueopsreview

import "testing"

func TestReviewPlanBindingSelectsLinkedAndHashesFullStagedContent(t *testing.T) {
	if !HasLinkedReviewPlan(" plan.md ") || HasLinkedReviewPlan(" ") {
		t.Fatal("linked plan selection changed")
	}
	digest, err := StagedReviewPlanDigest("abc")
	if err != nil || digest != "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad" {
		t.Fatalf("digest=%s err=%v", digest, err)
	}
	for _, body := range []string{"", " ", "\n\t"} {
		if _, err := StagedReviewPlanDigest(body); err == nil {
			t.Fatal("empty staged plan accepted")
		}
	}
	other, err := StagedReviewPlanDigest(" abc ")
	if err != nil || other == digest {
		t.Fatal("staged content was trimmed before hashing")
	}
}
