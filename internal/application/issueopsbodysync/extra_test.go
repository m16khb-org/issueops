package issueopsbodysync

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

const (
	syncPRURL    = "https://github.com/o/r/pull/7"
	syncChildURL = "https://github.com/o/r/issues/99"
)

var errBoom = errors.New("boom")

// scriptedProvider lets each test choose the state, failures and write result
// the provider reports.
type scriptedProvider struct {
	body, state             string
	childVerified           bool
	hierarchyErr, readErr   error
	replaceErr              error
	skipUpdated             bool // write reports Updated=false
	readbackSHA             string
	reads, writes, previews int
	hierarchyCalls          int
	lastRead                port.IssueProviderArtifactBodyRequest
}

func (p *scriptedProvider) Name() string { return "github" }

func (p *scriptedProvider) ReadArtifactBody(_ context.Context, req port.IssueProviderArtifactBodyRequest) (port.IssueProviderArtifactBody, error) {
	p.reads++
	p.lastRead = req
	if p.readErr != nil {
		return port.IssueProviderArtifactBody{}, p.readErr
	}
	return port.IssueProviderArtifactBody{Provider: "github", Kind: req.Kind, URL: req.URL, Body: p.body, State: p.state}, nil
}

func (p *scriptedProvider) ReplaceArtifactBody(_ context.Context, req port.IssueProviderReplaceArtifactBodyRequest) (port.IssueProviderReplaceArtifactBodyResult, error) {
	if p.replaceErr != nil {
		return port.IssueProviderReplaceArtifactBodyResult{}, p.replaceErr
	}
	if !req.Confirm {
		p.previews++
		return port.IssueProviderReplaceArtifactBodyResult{OK: true, Preview: "dry-run"}, nil
	}
	p.writes++
	sha := bodysync.SHA256Body(req.Body)
	if p.readbackSHA != "" {
		sha = p.readbackSHA
	}
	p.body = req.Body
	return port.IssueProviderReplaceArtifactBodyResult{OK: true, Updated: !p.skipUpdated, VerifiedBodySHA256: sha}, nil
}

func (p *scriptedProvider) VerifyChildHierarchy(context.Context, port.IssueProviderChildHierarchyRequest) (port.IssueProviderChildHierarchyResult, error) {
	p.hierarchyCalls++
	if p.hierarchyErr != nil {
		return port.IssueProviderChildHierarchyResult{}, p.hierarchyErr
	}
	return port.IssueProviderChildHierarchyResult{OK: true, Verified: p.childVerified}, nil
}

// denyAfter authorizes the first n calls, then refuses.
type denyAfter struct {
	allowed, calls int
}

func (a *denyAfter) Authorize(context.Context, issueops.IssueOpsRecord, issueops.IssueOpsActor) error {
	a.calls++
	if a.calls > a.allowed {
		return errBoom
	}
	return nil
}

type failingRepository struct {
	fakeRepository
	readErr, updateErr error
}

func (r *failingRepository) Read(ctx context.Context, id string) (issueops.IssueOpsRecord, error) {
	if r.readErr != nil {
		return issueops.IssueOpsRecord{}, r.readErr
	}
	return r.fakeRepository.Read(ctx, id)
}

func (r *failingRepository) Update(ctx context.Context, id string, transition RecordTransition) (issueops.IssueOpsRecord, error) {
	if r.updateErr != nil {
		return issueops.IssueOpsRecord{}, r.updateErr
	}
	return r.fakeRepository.Update(ctx, id, transition)
}

func syncNow() time.Time { return time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC) }

func issueRecord(syncs ...issueops.IssueOpsRemoteBodySync) issueops.IssueOpsRecord {
	return issueops.IssueOpsRecord{OK: true, ID: "io-sync", IssueURL: syncIssueURL, BodySyncs: syncs}
}

func prRecord(generation uint64) issueops.IssueOpsRecord {
	record := issueRecord()
	record.RemoteArtifact = &issueops.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: syncPRURL, VerifiedAt: "2026-10-04T00:00:00Z"}
	record.Execution = &issueops.Execution{Lease: issueops.WriteLease{Generation: generation}}
	return record
}

func issueSync(sha string) issueops.IssueOpsRemoteBodySync {
	return issueops.IssueOpsRemoteBodySync{Kind: "issue", URL: syncIssueURL, ToSHA256: sha, SyncedAt: "2026-10-01T00:00:00Z"}
}

func newScripted(record issueops.IssueOpsRecord, provider *scriptedProvider, authority Authority) (*Service, *failingRepository) {
	repository := &failingRepository{fakeRepository: fakeRepository{record: record}}
	return NewService(repository, provider, authority, syncNow), repository
}

func issueCommand(confirm bool) bodysynccontract.Command {
	return bodysynccontract.Command{Kind: bodysynccontract.KindIssue, ProposedBody: syncNewBody, Confirm: confirm}
}

func TestSyncRejectsAnUnusableProposalBeforeAnyIORead(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody}
	repository := &failingRepository{readErr: errBoom}
	service := NewService(repository, provider, allowAll{}, syncNow)
	for name, body := range map[string]string{
		"empty":          "  \n",
		"managed marker": syncNewBody + "\n<!-- issueops:completion:start -->",
	} {
		t.Run(name, func(t *testing.T) {
			cmd := issueCommand(false)
			cmd.ProposedBody = body
			_, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
			if err == nil || errors.Is(err, errBoom) {
				t.Fatalf("err = %v, want a proposal rejection raised before the repository read", err)
			}
		})
	}
	if provider.reads != 0 {
		t.Fatalf("provider reads = %d, want 0", provider.reads)
	}
}

func TestSyncPropagatesARepositoryReadFailure(t *testing.T) {
	service := NewService(&failingRepository{readErr: errBoom}, &scriptedProvider{}, allowAll{}, syncNow)
	_, _, err := service.Sync(t.Context(), "io-sync", issueCommand(false), issueops.IssueOpsActor{})
	if !errors.Is(err, errBoom) {
		t.Fatalf("err = %v", err)
	}
}

func TestSyncRefusesAnUnauthorizedActorBeforeReadingTheRemote(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody}
	service, _ := newScripted(issueRecord(), provider, &denyAfter{})
	record, _, err := service.Sync(t.Context(), "io-sync", issueCommand(false), issueops.IssueOpsActor{})
	if !errors.Is(err, errBoom) || record.OK || provider.reads != 0 {
		t.Fatalf("err=%v ok=%v reads=%d", err, record.OK, provider.reads)
	}
}

func TestSyncTargetResolutionFailures(t *testing.T) {
	tests := []struct {
		name   string
		record issueops.IssueOpsRecord
		cmd    bodysynccontract.Command
		want   string
	}{
		{"issue without a linked issue", issueops.IssueOpsRecord{OK: true}, issueCommand(false), "before the cycle has a linked issue"},
		{"unsupported kind", issueRecord(), bodysynccontract.Command{Kind: "wiki", ProposedBody: syncNewBody}, `unsupported body sync kind "wiki"`},
		{"pr without a verified artifact", issueRecord(), bodysynccontract.Command{Kind: bodysynccontract.KindPR, ProposedBody: syncNewBody}, "verified remote artifact"},
		{"pr without a lease", func() issueops.IssueOpsRecord { r := prRecord(1); r.Execution = nil; return r }(), bodysynccontract.Command{Kind: bodysynccontract.KindPR, ProposedBody: syncNewBody, ExpectedGeneration: 1}, "without an execution lease"},
		{"pr with a stale generation", prRecord(3), bodysynccontract.Command{Kind: bodysynccontract.KindPR, ProposedBody: syncNewBody, ExpectedGeneration: 2}, "stale lease generation: current=3 expected=2"},
		{"template for another artifact", issueRecord(), bodysynccontract.Command{Kind: bodysynccontract.KindIssue, ProposedBody: syncNewBody, Template: "pr"}, `template "pr" does not apply`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{body: syncLiveBody, state: "open"}
			service, _ := newScripted(tt.record, provider, allowAll{})
			_, _, err := service.Sync(t.Context(), "io-sync", tt.cmd, issueops.IssueOpsActor{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if provider.reads != 0 {
				t.Fatalf("target resolution must fail before a remote read, reads=%d", provider.reads)
			}
		})
	}
}

func TestSyncConfirmRefusesAnUnreadableProposalButPreviewReportsIt(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody}
	service, repository := newScripted(issueRecord(issueSync(bodysync.SHA256Body(syncLiveBody))), provider, allowAll{})
	cmd := issueCommand(true)
	cmd.ProposedBody = "그냥 한 줄"
	cmd.ExpectedBodySHA256 = bodysync.SHA256Body(syncLiveBody)
	_, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err == nil || !strings.Contains(err.Error(), "readability check failed") {
		t.Fatalf("confirm err = %v", err)
	}
	if provider.reads != 0 || provider.writes != 0 || repository.updates != 0 {
		t.Fatalf("reads=%d writes=%d updates=%d", provider.reads, provider.writes, repository.updates)
	}
	cmd.Confirm = false
	_, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatalf("preview must not be blocked by readability: %v", err)
	}
	if result.Readability == nil || result.Readability.OK {
		t.Fatalf("preview readability = %+v, want a failing report", result.Readability)
	}
}

func TestSyncReadFailuresAreReturned(t *testing.T) {
	t.Run("child hierarchy lookup", func(t *testing.T) {
		provider := &scriptedProvider{hierarchyErr: errBoom}
		service, _ := newScripted(issueRecord(), provider, allowAll{})
		cmd := issueCommand(false)
		cmd.URL = syncChildURL
		_, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
		if !errors.Is(err, errBoom) || provider.reads != 0 {
			t.Fatalf("err=%v reads=%d", err, provider.reads)
		}
	})
	t.Run("body read", func(t *testing.T) {
		provider := &scriptedProvider{readErr: errBoom}
		service, _ := newScripted(issueRecord(), provider, allowAll{})
		_, _, err := service.Sync(t.Context(), "io-sync", issueCommand(false), issueops.IssueOpsActor{})
		if !errors.Is(err, errBoom) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestSyncReadsAVerifiedChildAsAChildArtifact(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody, childVerified: true}
	service, _ := newScripted(issueRecord(), provider, allowAll{})
	cmd := issueCommand(false)
	cmd.URL = syncChildURL
	_, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != bodysynccontract.KindChild || result.URL != syncChildURL || provider.lastRead.Kind != bodysynccontract.KindChild || provider.lastRead.URL != syncChildURL {
		t.Fatalf("result=%+v read=%+v", result, provider.lastRead)
	}
}

func TestSyncRefusesAPublicationThatIsNotOpen(t *testing.T) {
	tests := []struct {
		state, want string
	}{
		{"merged", "(state=merged)"},
		{"closed", "(state=closed)"},
		{"", "without an observed artifact state"},
	}
	for _, tt := range tests {
		t.Run("state "+tt.state, func(t *testing.T) {
			provider := &scriptedProvider{body: syncLiveBody, state: tt.state}
			service, _ := newScripted(prRecord(2), provider, allowAll{})
			_, _, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{Kind: bodysynccontract.KindPR, ProposedBody: syncNewBody, ExpectedGeneration: 2}, issueops.IssueOpsActor{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if provider.previews != 0 || provider.writes != 0 {
				t.Fatalf("previews=%d writes=%d", provider.previews, provider.writes)
			}
		})
	}
}

func TestSyncPreviewReportsDriftBaselineAndAgeWithoutWriting(t *testing.T) {
	tests := []struct {
		name      string
		record    issueops.IssueOpsRecord
		wantDrift bodysynccontract.Drift
		wantSHA   string
		wantAt    string
		wantAge   int
	}{
		{"recorded sync matches the live body", issueRecord(issueSync(bodysync.SHA256Body(syncLiveBody))), bodysynccontract.DriftStale, bodysync.SHA256Body(syncLiveBody), "2026-10-01T00:00:00Z", 6},
		{"recorded sync differs from the live body", issueRecord(issueSync(strings.Repeat("a", 64))), bodysynccontract.DriftRemoteEdited, strings.Repeat("a", 64), "2026-10-01T00:00:00Z", 6},
		{"issue-create intent is the baseline", func() issueops.IssueOpsRecord {
			r := issueRecord()
			r.IssueCreateIntent = &issueops.IssueOpsIssueCreateIntent{CanonicalURL: syncIssueURL, BodySHA256: bodysync.SHA256Body(syncLiveBody), UpdatedAt: "2026-10-05T00:00:00Z"}
			return r
		}(), bodysynccontract.DriftStale, bodysync.SHA256Body(syncLiveBody), "2026-10-05T00:00:00Z", 2},
		{"no baseline at all", issueRecord(), bodysynccontract.DriftStale, "", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{body: syncLiveBody}
			service, repository := newScripted(tt.record, provider, allowAll{})
			record, result, err := service.Sync(t.Context(), "io-sync", issueCommand(false), issueops.IssueOpsActor{})
			if err != nil {
				t.Fatal(err)
			}
			if result.Drift != tt.wantDrift || result.RecordedBodySHA256 != tt.wantSHA || result.RecordedAt != tt.wantAt || result.AgeDays != tt.wantAge {
				t.Fatalf("result = %+v", result)
			}
			if result.Preview != "dry-run" || result.Updated || result.Confirm {
				t.Fatalf("preview result = %+v", result)
			}
			if result.ExpectedBodySHA256 != bodysync.SHA256Body(syncLiveBody) || result.RemoteBodySHA256 != result.ExpectedBodySHA256 {
				t.Fatalf("digests = %+v", result)
			}
			if result.LiveReadability == nil || !result.LiveReadability.OK || len(result.LiveReadability.Critical) != 0 {
				t.Fatalf("live readability must be warning-only: %+v", result.LiveReadability)
			}
			if !record.OK || provider.writes != 0 || repository.updates != 0 {
				t.Fatalf("preview wrote: ok=%v writes=%d updates=%d", record.OK, provider.writes, repository.updates)
			}
		})
	}
}

func TestSyncPublicationBaselineIsTheArtifactVerificationTime(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody, state: "open"}
	service, _ := newScripted(prRecord(2), provider, allowAll{})
	_, result, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{Kind: bodysynccontract.KindPR, ProposedBody: syncNewBody, ExpectedGeneration: 2}, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != "pr" || result.URL != syncPRURL || result.RecordedBodySHA256 != "" || result.RecordedAt != "2026-10-04T00:00:00Z" || result.AgeDays != 3 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSyncPreviewFailsWhenTheProviderPreviewFails(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody, replaceErr: errBoom}
	service, _ := newScripted(issueRecord(), provider, allowAll{})
	record, result, err := service.Sync(t.Context(), "io-sync", issueCommand(false), issueops.IssueOpsActor{})
	if !errors.Is(err, errBoom) || record.OK || result.Preview != "" {
		t.Fatalf("record=%+v result=%+v err=%v", record, result, err)
	}
}

func TestSyncConfirmRefusals(t *testing.T) {
	liveSHA := bodysync.SHA256Body(syncLiveBody)
	tests := []struct {
		name     string
		record   issueops.IssueOpsRecord
		expected string
		want     string
	}{
		{"missing expected digest", issueRecord(issueSync(liveSHA)), "", "--expected-body-sha256 is required"},
		{"outside edit without acceptance", issueRecord(issueSync(strings.Repeat("a", 64))), liveSHA, "edited outside the harness"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{body: syncLiveBody}
			service, repository := newScripted(tt.record, provider, allowAll{})
			cmd := issueCommand(true)
			cmd.ExpectedBodySHA256 = tt.expected
			_, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v, want %q", err, tt.want)
			}
			if provider.writes != 0 || repository.updates != 0 {
				t.Fatalf("writes=%d updates=%d", provider.writes, repository.updates)
			}
		})
	}
}

func TestSyncConfirmAcceptsAnOutsideEditWhenAcknowledged(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody}
	service, _ := newScripted(issueRecord(issueSync(strings.Repeat("a", 64))), provider, allowAll{})
	cmd := issueCommand(true)
	cmd.ExpectedBodySHA256 = bodysync.SHA256Body(syncLiveBody)
	cmd.AcceptRemoteEdits = true
	_, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Updated || !result.AcceptRemoteEdits || provider.writes != 1 {
		t.Fatalf("result=%+v writes=%d", result, provider.writes)
	}
}

func TestSyncConfirmOfAnIdenticalBodyIsASuccessfulNoOp(t *testing.T) {
	provider := &scriptedProvider{body: syncNewBody + "\r\n"}
	service, repository := newScripted(issueRecord(issueSync(bodysync.SHA256Body(syncNewBody))), provider, allowAll{})
	cmd := issueCommand(true)
	cmd.ExpectedBodySHA256 = bodysync.SHA256Body(syncNewBody)
	record, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if !record.OK || result.Updated || result.Drift != bodysynccontract.DriftInSync || provider.writes != 0 || repository.updates != 0 {
		t.Fatalf("record.OK=%v result=%+v writes=%d updates=%d", record.OK, result, provider.writes, repository.updates)
	}
}

func TestSyncConfirmFailsWhenTheWriteIsNotVerified(t *testing.T) {
	liveSHA := bodysync.SHA256Body(syncLiveBody)
	tests := []struct {
		name     string
		provider *scriptedProvider
		check    func(error) bool
	}{
		{"provider write error", &scriptedProvider{body: syncLiveBody, replaceErr: errBoom}, func(err error) bool { return errors.Is(err, errBoom) }},
		{"replacement not applied", &scriptedProvider{body: syncLiveBody, skipUpdated: true}, func(err error) bool {
			return strings.Contains(err.Error(), "did not report the body replacement as applied")
		}},
		{"readback differs", &scriptedProvider{body: syncLiveBody, readbackSHA: strings.Repeat("b", 64)}, func(err error) bool { return strings.Contains(err.Error(), "readback does not match") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, repository := newScripted(issueRecord(issueSync(liveSHA)), tt.provider, allowAll{})
			cmd := issueCommand(true)
			cmd.ExpectedBodySHA256 = liveSHA
			record, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
			if err == nil || !tt.check(err) {
				t.Fatalf("err = %v", err)
			}
			if record.OK || result.Updated || repository.updates != 0 {
				t.Fatalf("an unverified write must not stamp a baseline: record.OK=%v updated=%v updates=%d", record.OK, result.Updated, repository.updates)
			}
		})
	}
}

func TestSyncConfirmRecordsAPublicationBaselineWithItsGeneration(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody, state: "opened"}
	service, _ := newScripted(prRecord(4), provider, allowAll{})
	record, result, err := service.Sync(t.Context(), "io-sync", bodysynccontract.Command{
		Kind: bodysynccontract.KindPR, ProposedBody: syncPRBody, Confirm: true,
		ExpectedGeneration: 4, ExpectedBodySHA256: bodysync.SHA256Body(syncLiveBody),
	}, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	last := record.BodySyncs[len(record.BodySyncs)-1]
	if last.Generation != 4 || last.Kind != "pr" || last.URL != syncPRURL || last.SyncedAt != "2026-10-07T00:00:00Z" || record.UpdatedAt != last.SyncedAt {
		t.Fatalf("baseline = %+v updatedAt=%q", last, record.UpdatedAt)
	}
	if result.Drift != bodysynccontract.DriftInSync || result.ExpectedBodySHA256 != last.ToSHA256 || result.RemoteBodySHA256 != last.ToSHA256 {
		t.Fatalf("result = %+v", result)
	}
}

func TestSyncConfirmIssueBaselineCarriesNoGeneration(t *testing.T) {
	provider := &scriptedProvider{body: syncLiveBody}
	service, _ := newScripted(issueRecord(issueSync(bodysync.SHA256Body(syncLiveBody))), provider, allowAll{})
	cmd := issueCommand(true)
	cmd.ExpectedBodySHA256 = bodysync.SHA256Body(syncLiveBody)
	cmd.ExpectedGeneration = 9
	record, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if got := record.BodySyncs[len(record.BodySyncs)-1].Generation; got != 0 {
		t.Fatalf("issue baseline generation = %d, want 0", got)
	}
}

func TestSyncConfirmBaselineReplacesAliasesAndCapsHistory(t *testing.T) {
	liveSHA := bodysync.SHA256Body(syncLiveBody)
	var syncs []issueops.IssueOpsRemoteBodySync
	for i := range issueops.MaxIssueOpsBodySyncs {
		syncs = append(syncs, issueops.IssueOpsRemoteBodySync{Kind: "child", URL: "https://github.com/o/r/issues/" + string(rune('a'+i)), ToSHA256: "x"})
	}
	// 같은 산출물의 이전 기준선은 별칭(끝 슬래시) 형태로 맨 뒤에 둔다. 교체 필터가 없으면 상한 자르기로도 사라지지 않는다.
	prior := issueSync(liveSHA)
	prior.URL += "/"
	syncs = append(syncs, prior)
	provider := &scriptedProvider{body: syncLiveBody}
	service, _ := newScripted(issueRecord(syncs...), provider, allowAll{})
	cmd := issueCommand(true)
	cmd.ExpectedBodySHA256 = liveSHA
	record, _, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if len(record.BodySyncs) != issueops.MaxIssueOpsBodySyncs {
		t.Fatalf("baselines = %d, want the cap %d", len(record.BodySyncs), issueops.MaxIssueOpsBodySyncs)
	}
	for _, entry := range record.BodySyncs[:len(record.BodySyncs)-1] {
		if strings.TrimRight(entry.URL, "/") == syncIssueURL {
			t.Fatalf("the previous baseline of the same artifact must be replaced: %+v", entry)
		}
	}
	if record.BodySyncs[0].URL != "https://github.com/o/r/issues/b" {
		t.Fatalf("oldest unrelated baseline must be evicted first, head = %+v", record.BodySyncs[0])
	}
}

func TestSyncConfirmBaselineWriteFailuresKeepTheVerifiedResult(t *testing.T) {
	liveSHA := bodysync.SHA256Body(syncLiveBody)
	cmd := issueCommand(true)
	cmd.ExpectedBodySHA256 = liveSHA
	tests := []struct {
		name      string
		authority Authority
		updateErr error
	}{
		{"repository update fails", allowAll{}, errBoom},
		{"authority revoked before the baseline write", &denyAfter{allowed: 1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &scriptedProvider{body: syncLiveBody}
			service, repository := newScripted(issueRecord(issueSync(liveSHA)), provider, tt.authority)
			repository.updateErr = tt.updateErr
			record, result, err := service.Sync(t.Context(), "io-sync", cmd, issueops.IssueOpsActor{})
			if !errors.Is(err, errBoom) || record.OK {
				t.Fatalf("record.OK=%v err=%v", record.OK, err)
			}
			if !result.Updated || provider.writes != 1 {
				t.Fatalf("the remote write already happened and must stay reported: result=%+v writes=%d", result, provider.writes)
			}
		})
	}
}

const syncPRBody = `## 요약

본문 동기화가 PR 본문을 새 계약으로 바꿉니다. 리뷰어는 이 요약만 읽고 변경 이유를 압니다.

## 변경 내용

- 본문 교체 서비스가 병합 본문을 원격에 씁니다.

## 확인한 것

- 서비스 테스트로 교체 결과를 확인했습니다.

## 리뷰 포인트

- 관리 구간이 보존되는지 봐 주세요.`
