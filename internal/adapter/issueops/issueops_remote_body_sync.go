package issueops

import (
	"context"
	"errors"
	"fmt"
	"time"

	"issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
	bodysync "issueops/internal/domain/issueopsbodysync"
	"issueops/internal/port"
)

// SyncRemoteArtifactBody refreshes the body of an artifact this cycle already
// published, for the case the cycle moved on and the remote text did not.
//
// The write is fail-closed in three independent ways: the caller has to name
// the exact live body its proposal was built on, an artifact edited outside the
// harness needs a separate acknowledgement, and every managed block the harness
// maintains is spliced back in rather than replaced.
func SyncRemoteArtifactBody(
	ctx context.Context,
	stateRoot, id string,
	cmd bodysynccontract.Command,
	prov port.IssueProvider,
	actor IssueOpsActor,
) (issueops.IssueOpsRecord, bodysynccontract.Result, error) {
	if prov == nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("no issue provider configured")
	}
	reader, ok := prov.(port.IssueProviderArtifactBodyReader)
	if !ok {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider %q cannot read artifact bodies", prov.Name())
	}
	replacer, ok := prov.(port.IssueProviderArtifactBodyReplacer)
	if !ok {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider %q cannot replace artifact bodies", prov.Name())
	}
	// 쓸 수 없는 본문은 원격을 읽기 전에 거부한다. provider 왕복은 공짜가 아니다.
	if err := bodysync.ValidateProposal(cmd.ProposedBody); err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	record, err := ReadIssueOps(stateRoot, id)
	if err != nil {
		return record, bodysynccontract.Result{}, err
	}
	if err := validateExecutionMutation(record, &actor); err != nil {
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
		if err := verifyBodySyncChildHierarchy(ctx, prov, record, url); err != nil {
			return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
		}
	}
	live, err := reader.ReadArtifactBody(ctx, port.IssueProviderArtifactBodyRequest{
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
		OK: true, ID: record.ID, Provider: prov.Name(), Kind: kind, URL: url,
		Confirm:            cmd.Confirm,
		Drift:              plan.Drift,
		RecordedBodySHA256: baselineSHA,
		RemoteBodySHA256:   plan.RemoteBodySHA256,
		MergedBodySHA256:   plan.MergedBodySHA256,
		ExpectedBodySHA256: plan.RemoteBodySHA256,
		PreservedSections:  plan.PreservedSections,
		RecordedAt:         baselineAt,
		AgeDays:            bodysync.AgeDays(baselineAt, time.Now()),
		AcceptRemoteEdits:  cmd.AcceptRemoteEdits,
	}
	if !cmd.Confirm {
		preview, err := replacer.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
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
	written, err := replacer.ReplaceArtifactBody(ctx, port.IssueProviderReplaceArtifactBodyRequest{
		Repo: record.Repo, Kind: kind, URL: url, Body: plan.MergedBody, Confirm: true,
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, err
	}
	if !written.Updated {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf("provider did not report the body replacement as applied")
	}
	if written.VerifiedBodySHA256 != plan.MergedBodySHA256 {
		return issueops.IssueOpsRecord{OK: false}, bodysynccontract.Result{}, fmt.Errorf(
			"remote body readback does not match what was written (readback %s, intended %s)",
			written.VerifiedBodySHA256, plan.MergedBodySHA256)
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
		SyncedAt:   time.Now().UTC().Format(time.RFC3339Nano),
	}
	if bodysync.IsPublicationKind(kind) {
		entry.Generation = cmd.ExpectedGeneration
	}
	stamped, err := recordBodySync(ctx, stateRoot, id, entry, &actor)
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, result, err
	}
	return stamped, result, nil
}

func verifyBodySyncChildHierarchy(ctx context.Context, prov port.IssueProvider, record issueops.IssueOpsRecord, childURL string) error {
	verifier, ok := prov.(port.IssueProviderChildHierarchyVerifier)
	if !ok {
		return fmt.Errorf("provider %q cannot verify child hierarchy, so a child body cannot be synced", prov.Name())
	}
	result, err := verifier.VerifyChildHierarchy(ctx, port.IssueProviderChildHierarchyRequest{
		Repo: record.Repo, ParentIssueURL: record.IssueURL, ChildURL: childURL,
	})
	if err != nil {
		return err
	}
	if !result.Verified {
		return fmt.Errorf("%s is not a provider-native child of %s; sync it from the cycle that owns it", childURL, record.IssueURL)
	}
	return nil
}

// recordBodySync stores the new baseline under the record lock, keeping one
// entry per artifact so the list cannot grow without bound.
func recordBodySync(ctx context.Context, stateRoot, id string, entry issueops.IssueOpsRemoteBodySync, actor *IssueOpsActor) (issueops.IssueOpsRecord, error) {
	var stamped issueops.IssueOpsRecord
	err := withIssueOpsLock(ctx, stateRoot, id, func(context.Context) error {
		rec, readErr := ReadIssueOps(stateRoot, id)
		if readErr != nil {
			return readErr
		}
		if err := validateExecutionMutation(rec, actor); err != nil {
			return err
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
		var writeErr error
		stamped, writeErr = writeIssueOps(stateRoot, rec)
		return writeErr
	})
	if err != nil {
		return issueops.IssueOpsRecord{OK: false}, err
	}
	return stamped, nil
}
