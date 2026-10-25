package issueops

import (
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func covIZPendingIntent(t *testing.T) model.IssueOpsRecord {
	t.Helper()
	pending, err := BeginIssueCreateIntent(model.IssueOpsRecord{}, issueCreateRequestForTest())
	if err != nil {
		t.Fatal(err)
	}
	return pending
}

func TestBeginIssueCreateIntentRejectsLinkedRecordAndIncompleteRequests(t *testing.T) {
	linked := model.IssueOpsRecord{IssueURL: "https://github.com/acme/repo/issues/9"}
	if _, err := BeginIssueCreateIntent(linked, issueCreateRequestForTest()); err == nil || !strings.Contains(err.Error(), linked.IssueURL) {
		t.Fatalf("linked record accepted: %v", err)
	}
	for _, tc := range []struct {
		field  string
		mutate func(*model.IssueOpsIssueCreateIntentRequest)
	}{
		{"operation_id", func(r *model.IssueOpsIssueCreateIntentRequest) { r.OperationID = " " }},
		{"provider", func(r *model.IssueOpsIssueCreateIntentRequest) { r.Provider = "" }},
		{"project_authority", func(r *model.IssueOpsIssueCreateIntentRequest) { r.ProjectAuthority = "\t" }},
		{"title", func(r *model.IssueOpsIssueCreateIntentRequest) { r.Title = "" }},
		{"body_sha256", func(r *model.IssueOpsIssueCreateIntentRequest) { r.BodySHA256 = "" }},
		{"started_at", func(r *model.IssueOpsIssueCreateIntentRequest) { r.StartedAt = " " }},
	} {
		request := issueCreateRequestForTest()
		tc.mutate(&request)
		record, err := BeginIssueCreateIntent(model.IssueOpsRecord{}, request)
		if err == nil || !strings.Contains(err.Error(), tc.field+" is required") {
			t.Fatalf("%s: err=%v", tc.field, err)
		}
		if record.IssueCreateIntent != nil {
			t.Fatalf("%s: rejected request still created an intent", tc.field)
		}
	}
	short := issueCreateRequestForTest()
	short.BodySHA256 = strings.Repeat("a", 63)
	if _, err := BeginIssueCreateIntent(model.IssueOpsRecord{}, short); err == nil || !strings.Contains(err.Error(), "64 hex") {
		t.Fatalf("short digest accepted: %v", err)
	}
}

func TestBeginIssueCreateIntentNormalizesSealedFields(t *testing.T) {
	request := issueCreateRequestForTest()
	request.Provider = " github "
	request.Title = "  Issue creation  "
	request.BodySHA256 = " " + strings.Repeat("A", 64) + " "
	request.Assignees = []string{"alice"}
	started, err := BeginIssueCreateIntent(model.IssueOpsRecord{}, request)
	if err != nil {
		t.Fatal(err)
	}
	intent := started.IssueCreateIntent
	if intent.Provider != "github" || intent.Title != "Issue creation" || intent.BodySHA256 != strings.Repeat("a", 64) ||
		intent.Marker != "<!-- issueops:issue-create:"+request.OperationID+" -->" || intent.Attempt != 1 ||
		intent.Status != model.IssueCreateIntentPending {
		t.Fatalf("intent not normalized: %+v", intent)
	}
	request.Labels[0] = "mutated"
	request.Assignees[0] = "mutated"
	if intent.Labels[0] != "quality" || intent.Assignees[0] != "alice" {
		t.Fatalf("intent aliases request slices: %+v", intent)
	}
}

func TestBeginIssueCreateIntentRetryClearsFailureAndRequiresIdenticalRequest(t *testing.T) {
	request := issueCreateRequestForTest()
	notInvoked, err := RecordIssueCreateOutcome(covIZPendingIntent(t), model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "cli missing", ObservedAt: "t1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.IssueOpsIssueCreateIntentRequest){
		"labels":    func(r *model.IssueOpsIssueCreateIntentRequest) { r.Labels = []string{"other"} },
		"assignees": func(r *model.IssueOpsIssueCreateIntentRequest) { r.Assignees = []string{"bob"} },
		"authority": func(r *model.IssueOpsIssueCreateIntentRequest) { r.ProjectAuthority = "github.com/other/repo" },
		"provider":  func(r *model.IssueOpsIssueCreateIntentRequest) { r.Provider = "gitlab" },
		"digest":    func(r *model.IssueOpsIssueCreateIntentRequest) { r.BodySHA256 = strings.Repeat("b", 64) },
		"operation": func(r *model.IssueOpsIssueCreateIntentRequest) { r.OperationID = strings.Repeat("1", 32) },
	} {
		changed := request
		mutate(&changed)
		if _, err := BeginIssueCreateIntent(notInvoked, changed); err == nil || !strings.Contains(err.Error(), "does not match sealed") {
			t.Fatalf("%s: changed retry accepted: %v", name, err)
		}
	}
	request.StartedAt = "t2"
	retried, err := BeginIssueCreateIntent(notInvoked, request)
	if err != nil {
		t.Fatal(err)
	}
	got := retried.IssueCreateIntent
	if got.Failure != "" || got.UpdatedAt != "t2" || got.StartedAt != notInvoked.IssueCreateIntent.StartedAt || got.Attempt != 2 {
		t.Fatalf("retry intent=%+v", got)
	}
}

func TestRecordIssueCreateOutcomeRejectsInvalidObservations(t *testing.T) {
	pending := covIZPendingIntent(t)
	authority := pending.IssueCreateIntent.ProjectAuthority
	if _, err := RecordIssueCreateOutcome(model.IssueOpsRecord{}, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked}, ""); err == nil || !strings.Contains(err.Error(), "no pending") {
		t.Fatalf("missing intent: %v", err)
	}
	completed := pending
	done := *pending.IssueCreateIntent
	done.Status = model.IssueCreateIntentCompleted
	completed.IssueCreateIntent = &done
	if _, err := RecordIssueCreateOutcome(completed, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentReceiptFailed, Failure: "x", ObservedAt: "t"}, ""); err == nil || !strings.Contains(err.Error(), "already completed") {
		t.Fatalf("completed intent mutated: %v", err)
	}
	observed := model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentURLObserved, CanonicalURL: "https://github.com/acme/repo/issues/3", Failure: "receipt pending", ObservedAt: "t"}
	for _, wrong := range []string{"", "github.com/other/repo"} {
		if _, err := RecordIssueCreateOutcome(pending, observed, wrong); err == nil || !strings.Contains(err.Error(), "does not match sealed authority") {
			t.Fatalf("authority %q accepted: %v", wrong, err)
		}
	}
	if _, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentInvokedUnknown, ObservedAt: "t"}, ""); err == nil || !strings.Contains(err.Error(), "requires failure") {
		t.Fatalf("failure-less unknown outcome accepted: %v", err)
	}
	if _, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "x", CanonicalURL: "https://github.com/acme/repo/issues/3", ObservedAt: "t"}, authority); err == nil || !strings.Contains(err.Error(), "must not have canonical_url") {
		t.Fatalf("not_invoked with URL accepted: %v", err)
	}
	if _, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "x"}, ""); err == nil || !strings.Contains(err.Error(), "updated_at is required") {
		t.Fatalf("outcome without observation time accepted: %v", err)
	}
	if _, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: "bogus", Failure: "x", ObservedAt: "t"}, ""); err == nil {
		t.Fatal("unknown status accepted")
	}
	if pending.IssueCreateIntent.Status != model.IssueCreateIntentPending || pending.IssueCreateIntent.CanonicalURL != "" {
		t.Fatalf("rejected outcomes mutated the intent: %+v", pending.IssueCreateIntent)
	}
}

func TestRecordIssueCreateOutcomeStoresObservedURLWithMatchingAuthority(t *testing.T) {
	pending := covIZPendingIntent(t)
	got, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{
		Status: model.IssueCreateIntentURLObserved, CanonicalURL: " https://github.com/acme/repo/issues/3 ", Failure: " receipt pending ", ObservedAt: " t9 ",
	}, pending.IssueCreateIntent.ProjectAuthority)
	if err != nil {
		t.Fatal(err)
	}
	intent := got.IssueCreateIntent
	if intent.Status != model.IssueCreateIntentURLObserved || intent.CanonicalURL != "https://github.com/acme/repo/issues/3" || intent.Failure != "receipt pending" || intent.UpdatedAt != "t9" {
		t.Fatalf("intent=%+v", intent)
	}
	if got.IssueURL != "" {
		t.Fatal("an observed-but-unverified URL must not link the record")
	}
}

func TestCompleteIssueCreateIntentRejectsUnsafeStates(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/1"
	pending := covIZPendingIntent(t)
	authority := pending.IssueCreateIntent.ProjectAuthority
	if _, err := CompleteIssueCreateIntent(model.IssueOpsRecord{}, url, "t", authority); err == nil || !strings.Contains(err.Error(), "no pending") {
		t.Fatalf("missing intent: %v", err)
	}
	notInvoked, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentNotInvoked, Failure: "x", ObservedAt: "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := CompleteIssueCreateIntent(notInvoked, url, "t", authority); err == nil || !strings.Contains(err.Error(), "not invoked") {
		t.Fatalf("not_invoked intent completed: %v", err)
	}
	for _, bad := range []string{"", "ftp://github.com/acme/repo/issues/1", "not a url"} {
		if _, err := CompleteIssueCreateIntent(pending, bad, "t", authority); err == nil {
			t.Fatalf("url %q accepted", bad)
		}
	}
	if _, err := CompleteIssueCreateIntent(pending, url, "t", ""); err == nil || !strings.Contains(err.Error(), "does not match sealed authority") {
		t.Fatalf("empty authority accepted: %v", err)
	}
	if pending.IssueURL != "" || pending.IssueCreateIntent.Status != model.IssueCreateIntentPending {
		t.Fatal("rejected completion mutated its input")
	}
}

func TestCompleteIssueCreateIntentFromRecoveredStatesClearsFailure(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/1"
	pending := covIZPendingIntent(t)
	authority := pending.IssueCreateIntent.ProjectAuthority
	failed, err := RecordIssueCreateOutcome(pending, model.IssueOpsIssueCreateOutcome{Status: model.IssueCreateIntentVerificationFailed, Failure: "marker mismatch", ObservedAt: "t"}, "")
	if err != nil {
		t.Fatal(err)
	}
	completed, err := CompleteIssueCreateIntent(failed, url, " t2 ", authority)
	if err != nil {
		t.Fatal(err)
	}
	intent := completed.IssueCreateIntent
	if intent.Failure != "" || intent.CanonicalURL != url || intent.UpdatedAt != "t2" || completed.IssueURL != url || failed.IssueCreateIntent.Failure != "marker mismatch" {
		t.Fatalf("completed=%+v original=%+v", intent, failed.IssueCreateIntent)
	}
}

func TestValidateIssueCreateTitleRejectsBlank(t *testing.T) {
	for _, title := range []string{"", "  \t\n"} {
		if err := ValidateIssueCreateTitle(title); err == nil {
			t.Fatalf("blank title %q accepted", title)
		}
	}
	if err := ValidateIssueCreateTitle(" real "); err != nil {
		t.Fatal(err)
	}
}

func TestValidateIssueURLRequiresHTTPURLWithHost(t *testing.T) {
	for _, bad := range []string{"", "ftp://github.com/a/b/issues/1", "github.com/a/b/issues/1", "https:///issues/1", "http://[::1", "file:///tmp/x"} {
		if err := ValidateIssueURL(bad); err == nil {
			t.Fatalf("url %q accepted", bad)
		}
	}
	for _, good := range []string{"https://github.com/a/b/issues/1", "http://gitlab.local/a/b/-/issues/2"} {
		if err := ValidateIssueURL(good); err != nil {
			t.Fatalf("url %q rejected: %v", good, err)
		}
	}
	if err := ValidateIssueURL(""); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("empty url error=%v", err)
	}
	// Probe: link-issue accepted these; each failed only at branch prepare
	// or a readback comparison.
	for bad, want := range map[string]string{
		"https://github.com/a/b/pull/3":                  "pull/merge request",
		"https://gitlab.com/g/p/-/merge_requests/3":      "pull/merge request",
		"https://github.com/a/b/issues/abc":              "issue number",
		"https://github.com/a/b/issues/3#issuecomment-1": "https://github.com/a/b/issues/3",
		"https://gitlab.com/g/p/-/issues/3?show=1":       "https://gitlab.com/g/p/-/issues/3",
	} {
		if err := ValidateIssueURL(bad); err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("url %q error=%v want %q", bad, err, want)
		}
	}
}

func TestApplyIssueLinkValidatesURLAndAdvancesReadyPlan(t *testing.T) {
	ready := model.IssueOpsRecord{Phase: model.IssueOpsPhaseGrill, Intent: &model.IssueOpsIntentContract{IntentClass: "trivial", RawRequest: "r", InterpretedIntent: "i", SuccessCriteria: []string{"done"}}}
	got, err := ApplyIssueLink(ready, " https://github.com/acme/repo/issues/4 ", "now")
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueURL != "https://github.com/acme/repo/issues/4" || got.Phase != model.IssueOpsPhasePlan || got.UpdatedAt != "now" {
		t.Fatalf("ready link=%+v", got)
	}
	if ready.IssueURL != "" || ready.Phase != model.IssueOpsPhaseGrill {
		t.Fatal("link mutated its input")
	}
	notReady, err := ApplyIssueLink(model.IssueOpsRecord{Phase: model.IssueOpsPhaseGrill}, "https://github.com/acme/repo/issues/4", "now")
	if err != nil {
		t.Fatal(err)
	}
	if notReady.Phase != model.IssueOpsPhaseGrill {
		t.Fatalf("phase advanced without intent: %s", notReady.Phase)
	}
	later := ready
	later.Phase = model.IssueOpsPhaseImplement
	relinked, err := ApplyIssueLink(later, "https://github.com/acme/repo/issues/4", "now")
	if err != nil {
		t.Fatal(err)
	}
	if relinked.Phase != model.IssueOpsPhaseImplement {
		t.Fatalf("link regressed phase to %s", relinked.Phase)
	}
	rejected, err := ApplyIssueLink(ready, "ftp://example.com/x", "now")
	if err == nil || rejected.IssueURL != "" || rejected.OK {
		t.Fatalf("invalid URL linked: %+v err=%v", rejected, err)
	}
}

func TestValidateRelatedLinkTypeRejectsChildAndUnknown(t *testing.T) {
	for _, kind := range []string{"", "child", "parent", "Blocks"} {
		err := ValidateRelatedLinkType(kind)
		if err == nil || !strings.Contains(err.Error(), "invalid link type") {
			t.Fatalf("type %q: err=%v", kind, err)
		}
	}
}

func TestValidateRelationURLNamesTheOffendingField(t *testing.T) {
	err := ValidateRelationURL("ftp://x/y", "target_url")
	if err == nil || err.Error() != "target_url must be an http(s) URL" {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateRelationURL("", "child_url"); err == nil || !strings.HasPrefix(err.Error(), "child_url ") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateRelationURL("https://github.com/a/b/issues/1", "child_url"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateChildLinkParentRequiresLinkedParent(t *testing.T) {
	if err := ValidateChildLinkParent(model.IssueOpsRecord{IssueURL: "  "}); err == nil {
		t.Fatal("unlinked parent accepted")
	}
	if err := ValidateChildLinkParent(model.IssueOpsRecord{IssueURL: "https://github.com/a/b/issues/1"}); err != nil {
		t.Fatal(err)
	}
}

func TestAppendIssueRelationRejectsDuplicatesPerType(t *testing.T) {
	const url = "https://github.com/acme/repo/issues/1"
	record := model.IssueOpsRecord{IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: url}, {Type: "blocks", URL: url}}}
	if _, err := AppendIssueRelation(record, model.IssueOpsIssueLink{Type: "child", URL: url}, "now"); err == nil || !strings.Contains(err.Error(), "child issue already linked") {
		t.Fatalf("duplicate child: %v", err)
	}
	if _, err := AppendIssueRelation(record, model.IssueOpsIssueLink{Type: "blocks", URL: url}, "now"); err == nil || !strings.Contains(err.Error(), "already linked as blocks") {
		t.Fatalf("duplicate relation: %v", err)
	}
	got, err := AppendIssueRelation(record, model.IssueOpsIssueLink{Type: "depends-on", URL: url}, "now")
	if err != nil {
		t.Fatalf("same URL with a different type must be allowed: %v", err)
	}
	if len(got.IssueLinks) != 3 || len(record.IssueLinks) != 2 {
		t.Fatalf("links=%d original=%d", len(got.IssueLinks), len(record.IssueLinks))
	}
}
