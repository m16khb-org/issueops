package issueopsbodysync

import (
	"context"
	"errors"
	"time"

	reportcontract "issueops/internal/contract/artifactreadability"
	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	"issueops/internal/domain/artifactreadability"
	"issueops/internal/domain/artifacttemplate"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

type Service struct {
	repository Repository
	provider   Provider
	authority  Authority
	now        func() time.Time
}

func NewService(repository Repository, provider Provider, authority Authority, now func() time.Time) *Service {
	return &Service{repository: repository, provider: provider, authority: authority, now: now}
}

func (s *Service) Sync(ctx context.Context, id string, cmd bodysynccontract.Command, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	// 쓸 수 없는 본문은 원격을 읽기 전에 거부한다. provider 왕복은 공짜가 아니다.
	if err := bodysync.ValidateProposal(cmd.ProposedBody); err != nil {
		return syncFailure(err)
	}
	record, err := s.repository.Read(ctx, id)
	if err != nil {
		return record, bodysynccontract.Result{}, err
	}
	if err := s.authorize(ctx, record, actor); err != nil {
		return syncFailure(err)
	}
	target, err := resolveSyncTarget(record, cmd)
	if err != nil {
		return syncFailure(err)
	}
	if cmd.Confirm && !target.readability.OK {
		return syncFailure(artifactreadability.RefusalError(target.readability))
	}
	live, err := s.readLiveBody(ctx, record, target)
	if err != nil {
		return syncFailure(err)
	}
	baselineSHA, baselineAt := bodysync.SelectBaseline(baselineSnapshot(record), target.kind, target.url)
	plan, err := bodysync.BuildPlan(baselineSHA, live.Body, cmd.ProposedBody)
	if err != nil {
		return syncFailure(err)
	}
	result := bodysynccontract.Result{
		OK: true, ID: record.ID, Provider: s.provider.Name(), Kind: target.kind, URL: target.url,
		Confirm:            cmd.Confirm,
		Drift:              plan.Drift,
		RecordedBodySHA256: baselineSHA,
		RemoteBodySHA256:   plan.RemoteBodySHA256,
		MergedBodySHA256:   plan.MergedBodySHA256,
		ExpectedBodySHA256: plan.RemoteBodySHA256,
		PreservedSections:  plan.PreservedSections,
		RecordedAt:         baselineAt,
		AgeDays:            bodysync.AgeDays(baselineAt, s.now()),
		AcceptRemoteEdits:  cmd.AcceptRemoteEdits,
		Readability:        &target.readability,
	}
	if !cmd.Confirm {
		return s.previewSync(ctx, record, target, live.Body, plan, result)
	}
	return s.confirmSync(ctx, id, record, cmd, actor, target, plan, result)
}

// syncFailure는 실패한 Sync가 돌려주는 값(OK=false 레코드, 빈 결과)을 만든다.
func syncFailure(err error) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
}

// syncTarget은 Sync가 어느 원격 산출물을 어떤 가독성 기준으로 다루는지 정리한 값이다.
type syncTarget struct {
	kind         string
	url          string
	artifactKind artifacttemplate.IssueOpsArtifactKind
	template     artifacttemplate.IssueOpsTemplateKind
	readability  reportcontract.Report
}

// resolveSyncTarget은 대상 산출물을 정하고 발행 세대와 제안 본문의 가독성을 검증한다.
func resolveSyncTarget(record issueops.IssueOpsRecord, cmd bodysynccontract.Command) (syncTarget, error) {
	snapshot := bodysync.TargetSnapshot{IssueURL: record.IssueURL}
	if record.RemoteArtifact != nil {
		snapshot.ArtifactURL, snapshot.ArtifactKind = record.RemoteArtifact.URL, record.RemoteArtifact.Kind
	}
	kind, url, err := bodysync.ResolveTarget(snapshot, cmd)
	if err != nil {
		return syncTarget{}, err
	}
	if bodysync.IsPublicationKind(kind) {
		current := uint64(0)
		if record.Execution != nil {
			current = record.Execution.Lease.Generation
		}
		if err := bodysync.ValidateGeneration(record.Execution != nil, current, cmd.ExpectedGeneration); err != nil {
			return syncTarget{}, err
		}
	}
	artifactKind := artifactreadability.BodySyncArtifactKind(kind)
	template, err := artifactreadability.BodySyncTemplate(artifactKind, cmd.Template, cmd.ProposedBody)
	if err != nil {
		return syncTarget{}, err
	}
	return syncTarget{
		kind: kind, url: url, artifactKind: artifactKind, template: template,
		readability: artifactreadability.CheckBodySyncReadability(artifactKind, template, cmd.ProposedBody),
	}, nil
}

// readLiveBody는 (자식이면 계층을 확인한 뒤) 원격 본문을 읽고 닫힌 발행 대상을 거부한다.
func (s *Service) readLiveBody(ctx context.Context, record issueops.IssueOpsRecord, target syncTarget) (port.IssueProviderArtifactBody, error) {
	if target.kind == bodysynccontract.KindChild {
		if err := s.verifyChild(ctx, record, target.url); err != nil {
			return port.IssueProviderArtifactBody{}, err
		}
	}
	live, err := s.provider.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{
		Repo: record.Repo, Kind: target.kind, URL: target.url,
	})
	if err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	if err := bodysync.RejectClosedPublication(target.kind, live.State); err != nil {
		return port.IssueProviderArtifactBody{}, err
	}
	return live, nil
}

// baselineSnapshot은 레코드에 남은 이전 동기화·생성 의도·산출물 검증 시각을 기준선 후보로 모은다.
func baselineSnapshot(record issueops.IssueOpsRecord) bodysync.BaselineSnapshot {
	baseline := bodysync.BaselineSnapshot{Entries: make([]bodysync.BaselineEntry, len(record.BodySyncs))}
	for index, entry := range record.BodySyncs {
		baseline.Entries[index] = bodysync.BaselineEntry{URL: entry.URL, SHA256: entry.ToSHA256, SyncedAt: entry.SyncedAt}
	}
	if record.IssueCreateIntent != nil {
		baseline.IssueCreateURL = record.IssueCreateIntent.CanonicalURL
		baseline.IssueCreateSHA256 = record.IssueCreateIntent.BodySHA256
		baseline.IssueCreateAt = record.IssueCreateIntent.UpdatedAt
	}
	if record.RemoteArtifact != nil {
		baseline.ArtifactVerifiedAt = record.RemoteArtifact.VerifiedAt
	}
	return baseline
}

// previewSync는 원격을 바꾸지 않고 병합 결과 미리보기만 결과에 채운다.
func (s *Service) previewSync(ctx context.Context, record issueops.IssueOpsRecord, target syncTarget, liveBody string, plan bodysynccontract.Plan, result bodysynccontract.Result) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	liveReadability := artifactreadability.WarningOnly(artifactreadability.CheckBodySyncReadability(target.artifactKind, target.template, liveBody))
	result.LiveReadability = &liveReadability
	preview, err := s.provider.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
		Repo: record.Repo, Kind: target.kind, URL: target.url, Body: plan.MergedBody,
	})
	if err != nil {
		return syncFailure(err)
	}
	result.Preview = preview.Preview
	return record, result, nil
}

// confirmSync는 병합 본문을 원격에 쓰고 읽어 되돌려 확인한 뒤 새 기준선을 기록한다.
func (s *Service) confirmSync(ctx context.Context, id string, record issueops.IssueOpsRecord, cmd bodysynccontract.Command, actor issueops.IssueOpsActor, target syncTarget, plan bodysynccontract.Plan, result bodysynccontract.Result) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	if err := bodysync.ValidateWrite(plan, cmd.ExpectedBodySHA256, cmd.AcceptRemoteEdits); err != nil {
		if errors.Is(err, bodysync.ErrAlreadyInSync) {
			// 이미 같은 본문이면 provider를 건드리지 않는다. 성공이지 실패가 아니다.
			return record, result, nil
		}
		return syncFailure(err)
	}
	written, err := s.provider.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
		Repo: record.Repo, Kind: target.kind, URL: target.url, Body: plan.MergedBody, Confirm: true,
	})
	if err != nil {
		return syncFailure(err)
	}
	if err := bodysync.ValidateReadback(written.Updated, written.VerifiedBodySHA256, plan.MergedBodySHA256); err != nil {
		return syncFailure(err)
	}
	result.Updated = true
	result.RemoteBodySHA256 = written.VerifiedBodySHA256
	// 두 필드 모두 "지금 원격 본문"을 가리켜야 다음 confirm이 그대로 재사용한다.
	result.ExpectedBodySHA256 = written.VerifiedBodySHA256
	result.Drift = bodysynccontract.DriftInSync

	entry := issueops.IssueOpsRemoteBodySync{
		Kind: target.kind, URL: target.url,
		FromSHA256: plan.RemoteBodySHA256,
		ToSHA256:   written.VerifiedBodySHA256,
		SyncedAt:   s.now().UTC().Format(time.RFC3339Nano),
	}
	if bodysync.IsPublicationKind(target.kind) {
		entry.Generation = cmd.ExpectedGeneration
	}
	stamped, err := s.recordBaseline(ctx, id, entry, actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	return stamped, result, nil
}

// authorize admits the lease holder, or anyone once the cycle is completed
// and released (bodysync.PostCompletion). Confirm re-reads the record and
// calls it on entry, so a reseed between preview and confirm is refused; the
// re-check inside the locked update only stops the baseline record, not a
// remote write already sent.
func (s *Service) authorize(ctx context.Context, record issueops.IssueOpsRecord, actor issueops.IssueOpsActor) error {
	if execution := record.Execution; execution != nil &&
		bodysync.PostCompletion(string(record.Phase), string(execution.Lease.Status), execution.Completion != nil) {
		return nil
	}
	return s.authority.Authorize(ctx, record, actor)
}

func (s *Service) verifyChild(ctx context.Context, record issueops.IssueOpsRecord, childURL string) error {
	result, err := s.provider.VerifyChildHierarchy(ctx, port.IssueProviderChildHierarchyRequest{Repo: record.Repo, ParentIssueURL: record.IssueURL, ChildURL: childURL})
	if err != nil {
		return err
	}
	return bodysync.ValidateChildHierarchy(result.Verified, childURL, record.IssueURL)
}

func (s *Service) recordBaseline(ctx context.Context, id string, entry issueops.IssueOpsRemoteBodySync, actor issueops.IssueOpsActor) (issueops.IssueOpsRecord, error) {
	return s.repository.Update(ctx, id, func(rec issueops.IssueOpsRecord) (issueops.IssueOpsRecord, error) {
		if err := s.authorize(ctx, rec, actor); err != nil {
			return rec, err
		}
		urls := make([]string, len(rec.BodySyncs))
		for index, existing := range rec.BodySyncs {
			urls[index] = existing.URL
		}
		indices := bodysync.RetainedBaselineIndices(urls, entry.URL, issueops.MaxIssueOpsBodySyncs)
		kept := make([]issueops.IssueOpsRemoteBodySync, 0, len(indices)+1)
		for _, index := range indices {
			kept = append(kept, rec.BodySyncs[index])
		}
		rec.BodySyncs = append(kept, entry)
		rec.UpdatedAt = entry.SyncedAt
		return rec, nil
	})
}
