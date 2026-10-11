package issueopscleanup_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func closerFor(records *lockedChildCleanupRecords, provider func(string) (port.IssueProvider, error)) app.ChildrenCloser {
	return app.ChildrenCloser{
		Records:      records,
		Provider:     provider,
		VerifyMerged: func(model.IssueOpsRemoteArtifactVerification) error { return nil },
		Now:          func() time.Time { return time.Unix(123, 0) },
	}
}

func TestChildrenCloserWithoutMergeEvidenceRequiresReadableChildState(t *testing.T) {
	providerFailure, closeFailure := errors.New("no such provider"), errors.New("api down")
	childURL := "https://github.com/acme/repo/issues/2"
	cases := []struct {
		name     string
		link     model.IssueOpsIssueLink
		parent   string
		provider func(string) (port.IssueProvider, error)
		want     string
	}{
		{name: "provider cannot be determined", link: model.IssueOpsIssueLink{Type: "child", URL: "child-2"}, parent: "https://example.test/issues/1", provider: func(string) (port.IssueProvider, error) { t.Fatal("provider requested"); return nil, nil }, want: "cannot determine provider for child child-2"},
		{name: "provider construction fails", link: model.IssueOpsIssueLink{Type: "child", URL: childURL}, parent: "https://github.com/acme/repo/issues/1", provider: func(string) (port.IssueProvider, error) { return nil, providerFailure }, want: "no such provider"},
		{name: "child state probe fails", link: model.IssueOpsIssueLink{Type: "child", URL: childURL}, parent: "https://github.com/acme/repo/issues/1", provider: func(string) (port.IssueProvider, error) {
			return childCloseFunction{close: func(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
				return port.IssueProviderCloseChildResult{}, closeFailure
			}}, nil
		}, want: "cannot close child tasks without merge evidence: api down"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "parent", IssueURL: tc.parent, IssueLinks: []model.IssueOpsIssueLink{tc.link}}
			records := &lockedChildCleanupRecords{t: t, record: record}
			result, err := closerFor(records, tc.provider).Close(context.Background(), record.ID, true, true)
			if err == nil || !strings.Contains(err.Error(), tc.want) || result.ClosedCount != 0 || records.saves != 0 {
				t.Fatalf("result=%+v err=%v saves=%d", result, err, records.saves)
			}
		})
	}
}

func TestChildrenCloserProbesChildStateWithoutConfirmingClose(t *testing.T) {
	record := model.IssueOpsRecord{ID: "parent", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", IssueLinks: []model.IssueOpsIssueLink{
		{Type: "child", URL: "https://github.com/acme/repo/issues/2"},
		{Type: "related", URL: "https://github.com/acme/repo/issues/9"},
	}}
	var probes []port.IssueProviderCloseChildRequest
	provider := childCloseFunction{close: func(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
		probes = append(probes, req)
		return port.IssueProviderCloseChildResult{State: " CLOSED ", HierarchyVerified: true, AlreadyClosed: true}, nil
	}}
	records := &lockedChildCleanupRecords{t: t, record: record}
	result, err := closerFor(records, func(string) (port.IssueProvider, error) { return provider, nil }).Close(context.Background(), record.ID, true, false)
	if err != nil || result.EvidenceBasis != "children_already_closed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(probes) < 1 || probes[0].Confirm || probes[0].Repo != "/repo" || probes[0].ParentIssueURL != record.IssueURL || probes[0].ChildURL != "https://github.com/acme/repo/issues/2" {
		t.Fatalf("probe must be a non-confirming read of the child: %+v", probes)
	}
	for _, probe := range probes {
		if strings.HasSuffix(probe.ChildURL, "/9") {
			t.Fatalf("non-child link probed: %+v", probe)
		}
	}
}

func TestChildrenCloserReportsCanonicalChildIdentityFromProvider(t *testing.T) {
	childURL := "https://github.com/acme/repo/issues/2"
	for _, tc := range []struct {
		name         string
		provider     port.IssueProviderCloseChildResult
		wantURL      string
		wantProvider string
	}{
		{name: "provider canonicalizes", provider: port.IssueProviderCloseChildResult{ChildURL: "  https://github.com/acme/repo/issues/2?canonical ", Provider: " gh ", Closed: true, HierarchyVerified: true}, wantURL: "https://github.com/acme/repo/issues/2?canonical", wantProvider: "gh"},
		{name: "provider leaves identity blank", provider: port.IssueProviderCloseChildResult{Closed: true, HierarchyVerified: true}, wantURL: childURL, wantProvider: "github"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "parent", IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"}, IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: childURL}}}
			records := &lockedChildCleanupRecords{t: t, record: record}
			provider := childCloseFunction{close: func(port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
				return tc.provider, nil
			}}
			result, err := closerFor(records, func(string) (port.IssueProvider, error) { return provider, nil }).Close(context.Background(), record.ID, true, true)
			if err != nil || len(result.Children) != 1 || result.Children[0].URL != tc.wantURL || result.Children[0].Provider != tc.wantProvider {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if records.saves != 1 {
				t.Fatalf("saves=%d", records.saves)
			}
		})
	}
}

func TestChildrenCloserReportsUnresolvableChildProvider(t *testing.T) {
	providerFailure := errors.New("provider disabled")
	record := model.IssueOpsRecord{ID: "parent", IssueURL: "https://example.test/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: "https://github.com/acme/repo/pull/100"}, IssueLinks: []model.IssueOpsIssueLink{{Type: "child", URL: "child-2"}}}
	records := &lockedChildCleanupRecords{t: t, record: record}
	result, err := closerFor(records, func(string) (port.IssueProvider, error) { return nil, nil }).Close(context.Background(), record.ID, true, true)
	if err == nil || len(result.Children) != 1 || result.Children[0].Error != "cannot determine provider" || records.saves != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	record.IssueURL = "https://github.com/acme/repo/issues/1"
	record.IssueLinks = []model.IssueOpsIssueLink{{Type: "child", URL: "https://github.com/acme/repo/issues/2"}}
	records = &lockedChildCleanupRecords{t: t, record: record}
	result, err = closerFor(records, func(string) (port.IssueProvider, error) { return nil, providerFailure }).Close(context.Background(), record.ID, true, true)
	if !errors.Is(err, providerFailure) || len(result.Children) != 1 || result.Children[0].Error != providerFailure.Error() || result.Children[0].Provider != "github" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
