package issueopsbodysync

import (
	"context"
	"errors"
	"time"

	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
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
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	record, err := s.repository.Read(ctx, id)
	if err != nil {
		return record, bodysynccontract.Result{}, err
	}
	if err := s.authority.Authorize(ctx, record, actor); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	target := bodysync.TargetSnapshot{IssueURL: record.IssueURL}
	if record.RemoteArtifact != nil {
		target.ArtifactURL, target.ArtifactKind = record.RemoteArtifact.URL, record.RemoteArtifact.Kind
	}
	kind, url, err := bodysync.ResolveTarget(target, cmd)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if bodysync.IsPublicationKind(kind) {
		current := uint64(0)
		if record.Execution != nil {
			current = record.Execution.Lease.Generation
		}
		if err := bodysync.ValidateGeneration(record.Execution != nil, current, cmd.ExpectedGeneration); err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
	}
	if kind == bodysynccontract.KindChild {
		if err := s.verifyChild(ctx, record, url); err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
	}
	live, err := s.provider.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{
		Repo: record.Repo, Kind: kind, URL: url,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if err := bodysync.RejectClosedPublication(kind, live.State); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
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
	baselineSHA, baselineAt := bodysync.SelectBaseline(baseline, kind, url)
	plan, err := bodysync.BuildPlan(baselineSHA, live.Body, cmd.ProposedBody)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	result := bodysynccontract.Result{
		OK: true, ID: record.ID, Provider: s.provider.Name(), Kind: kind, URL: url,
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
	}
	if !cmd.Confirm {
		preview, err := s.provider.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
			Repo: record.Repo, Kind: kind, URL: url, Body: plan.MergedBody,
		})
		if err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
		result.Preview = preview.Preview
		return record, result, nil
	}
	if err := bodysync.ValidateWrite(plan, cmd.ExpectedBodySHA256, cmd.AcceptRemoteEdits); err != nil {
		if errors.Is(err, bodysync.ErrAlreadyInSync) {
			// 이미 같은 본문이면 provider를 건드리지 않는다. 성공이지 실패가 아니다.
			return record, result, nil
		}
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	written, err := s.provider.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
		Repo: record.Repo, Kind: kind, URL: url, Body: plan.MergedBody, Confirm: true,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if err := bodysync.ValidateReadback(written.Updated, written.VerifiedBodySHA256, plan.MergedBodySHA256); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	result.Updated = true
	result.RemoteBodySHA256 = written.VerifiedBodySHA256
	// 두 필드 모두 "지금 원격 본문"을 가리켜야 다음 confirm이 그대로 재사용한다.
	result.ExpectedBodySHA256 = written.VerifiedBodySHA256
	result.Drift = bodysynccontract.DriftInSync

	entry := issueops.IssueOpsRemoteBodySync{
		Kind: kind, URL: url,
		FromSHA256: plan.RemoteBodySHA256,
		ToSHA256:   written.VerifiedBodySHA256,
		SyncedAt:   s.now().UTC().Format(time.RFC3339Nano),
	}
	if bodysync.IsPublicationKind(kind) {
		entry.Generation = cmd.ExpectedGeneration
	}
	stamped, err := s.recordBaseline(ctx, id, entry, actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	return stamped, result, nil
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
		if err := s.authority.Authorize(ctx, rec, actor); err != nil {
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
