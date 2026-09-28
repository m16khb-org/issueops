package issueops

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"issueops/internal/adapter/issueops/artifactverify"
	"issueops/internal/adapter/issueops/implementation"
	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	publicationcontract "issueops/internal/contract/issueopspublication"
	publicationdomain "issueops/internal/domain/issueopspublication"
	"issueops/internal/domain/issueopsremote"
	"issueops/internal/domain/policy"
	"issueops/internal/port"
)

var externalIntentBucket = fmt.Sprintf("external_intent_v%d", issueops.IssueOpsSchemaVersion)

const (
	externalIntentRemotePR  = "remote_pr_create"
	remoteInvocationUnknown = "unknown"
)

type RemoteArtifactVerifyFunc func(issueops.IssueOpsRemoteArtifactVerificationRequest) error

type RemotePullRequestDependencies struct {
	Handler RemotePullRequestCreateHandler
}

type externalRemotePRPayload struct {
	SchemaVersion   int                                        `json:"schema_version"`
	OperationID     string                                     `json:"operation_id"`
	Generation      uint64                                     `json:"generation"`
	Provider        string                                     `json:"provider"`
	Kind            string                                     `json:"kind"`
	Request         port.IssueProviderCreatePullRequestRequest `json:"request"`
	InvocationState string                                     `json:"invocation_state"`
	RetryCount      int                                        `json:"retry_count"`
	KnownURL        string                                     `json:"known_url,omitempty"`
}

// CreateRemotePullRequest는 IssueOps v1의 유일한 PR/MR 생성 경로다. provider를
// 호출하기 전에 정확한 operation intent 하나를 영속화하며, 모호한 호출은 절대
// 재시도하지 않는다.
func CreateRemotePullRequest(ctx context.Context, stateRoot string, req RemotePullRequestRequest, deps RemotePullRequestDependencies) (port.IssueProviderCreatePullRequestResult, error) {
	if deps.Handler == nil {
		return port.IssueProviderCreatePullRequestResult{}, ErrRemotePullRequestCreateHandlerUnavailable
	}
	if req.Confirm {
		actor, err := normalizeNativeActor(req.Actor)
		if err != nil {
			return port.IssueProviderCreatePullRequestResult{}, err
		}
		req.Actor = actor
	}
	return deps.Handler(ctx, stateRoot, req)
}

func prepareRemotePullRequest(stateRoot string, req RemotePullRequestRequest) (issueops.IssueOpsRecord, port.IssueProviderCreatePullRequestRequest, string, error) {
	record, err := ReadIssueOps(stateRoot, req.ID)
	if err != nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	} else if provider != "github" {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote provider must be github or gitlab")
	}
	if record.Phase != issueops.IssueOpsPhasePR || record.RemoteArtifact != nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires phase pr and no existing remote artifact")
	}
	if req.Confirm {
		if record.Execution == nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires IssueOps execution v1")
		}
		if req.ExpectedGeneration == 0 || record.Execution.Lease.Generation != req.ExpectedGeneration {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("stale lease generation: current=%d expected=%d", record.Execution.Lease.Generation, req.ExpectedGeneration)
		}
		mutationActor := IssueOpsActor{
			Host: req.Actor.Host, SessionID: req.Actor.SessionID, AgentID: req.Actor.AgentID, CWD: req.CWD,
			NativeProcessAncestry: req.Actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(record, &mutationActor); err != nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", err
		}
		if record.Execution.Pending != nil {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("external intent is already pending; run execution reconcile")
		}
		// publication 하드 게이트: planner급 design-review 리뷰의 pass 기록 없이는
		// publication을 열지 않는다(설계 v5 WS5). execution이 있는 모든 모드가
		// 대상이다 — 9단계 재편에서 direct가 기본 경로가 되고 검증 단계가 이
		// 기록을 만든다.
		currentReviewFingerprint := implementation.ChangeFingerprint(record)
		if missing := implementationReviewMissing(record, currentReviewFingerprint); missing != "" {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires a pass implementation review (%s); record it with `issueops implementation-review record --id %s ...`", missing, record.ID)
		}
		// ai_slop_clean 선례(strict:59-61)와 동형: 리뷰가 fingerprint를 봉인했는데
		// 현재 값을 계산할 수 없으면 staleness 판정을 조용히 끄는 대신 거부한다.
		if currentReviewFingerprint == "" &&
			record.ImplementationReview != nil && record.ImplementationReview.ReviewedFingerprint != "" {
			return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create cannot verify implementation review freshness (current_fingerprint unavailable)")
		}
	}
	if record.BranchPrepare == nil {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires branch preparation")
	}
	// PR/MR은 코드가 있는 프로젝트에 만들어진다. 이슈가 다른 프로젝트에 있는
	// 사이클은 branch prepare가 봉인한 code project key를 쓰고, 봉인이 없으면
	// 이슈 프로젝트가 곧 코드 프로젝트다.
	projectKey := remote.EffectiveProjectKey(record.BranchPrepare.CodeProjectKey, record.IssueURL, provider)
	head, base := strings.TrimSpace(req.Head), strings.TrimSpace(req.Base)
	workspaceBranch := strings.TrimSpace(record.Branch)
	if record.Execution != nil {
		workspaceBranch = record.Execution.Workspace.Branch
	}
	if projectKey == "" || provider != strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider)) || head != workspaceBranch || base != strings.TrimSpace(record.BranchPrepare.BaseBranch) {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create request does not match execution workspace and linked issue authority")
	}
	title, body := strings.TrimSpace(req.Title), strings.TrimSpace(req.Body)
	if title == "" || len(title) > 1024 || len(body) > 1<<20 {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title is required and body must not exceed 1048576 bytes")
	}
	if policy.RedactFreeform(title) != title || policy.RedactFreeform(body) != body {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create title or body contains secret-like content")
	}
	labels, assignees := remote.CleanValues(req.Labels), remote.CleanValues(req.Assignees)
	if len(labels) == 0 || len(assignees) == 0 {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires canonical labels and assignees")
	}
	if invalid := remote.InvalidAssignee(assignees); invalid != "" {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create assignee must not be placeholder %q", invalid)
	}
	headSHA := ""
	if req.Confirm {
		headSHA = issueOpsCurrentHead(record)
	}
	if req.Confirm && !validExecutionHead(headSHA) {
		return issueops.IssueOpsRecord{}, port.IssueProviderCreatePullRequestRequest{}, "", fmt.Errorf("remote create requires a resolvable canonical worktree HEAD")
	}
	workingRoot := record.Repo
	if req.Confirm {
		workingRoot = record.Execution.Workspace.Root
	}
	return record, port.IssueProviderCreatePullRequestRequest{
		Repo: workingRoot, ProjectKey: projectKey, Title: title, Body: body,
		HeadBranch: head, BaseBranch: base, Labels: labels, Assignees: assignees,
		Draft: true, ExpectedHeadSHA: headSHA, Confirm: req.Confirm,
		Host: req.Actor.Host, SessionID: req.Actor.SessionID, AgentID: req.Actor.AgentID, CWD: req.CWD,
	}, kind, nil
}

func beginRemotePullRequestIntentWithOperationID(stateRoot string, expected issueops.IssueOpsRecord, actor issueops.NativeActor, cwd string, expectedGeneration uint64, providerReq port.IssueProviderCreatePullRequestRequest, provider, kind, operationID string, now func() time.Time) (issueops.IssueOpsRecord, externalRemotePRPayload, error) {
	if err := publicationdomain.ValidateOperationID(operationID); err != nil {
		return issueops.IssueOpsRecord{}, externalRemotePRPayload{}, err
	}
	marker := "<!-- issueops:issueops-v1 operation=" + operationID + " -->"
	providerReq.Body = strings.TrimSpace(providerReq.Body) + "\n\n" + marker
	payload := externalRemotePRPayload{
		SchemaVersion: issueops.IssueOpsSchemaVersion, OperationID: operationID, Provider: provider, Kind: kind,
		Request: providerReq, InvocationState: remoteInvocationUnknown,
	}
	var persisted issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, expected.ID, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, expected.ID)
		if err != nil {
			return err
		}
		mutationActor := IssueOpsActor{
			Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID, CWD: cwd,
			NativeProcessAncestry: actor.ProcessAncestry,
		}
		if err := validateExecutionMutation(current, &mutationActor); err != nil {
			return err
		}
		facts := publicationdomain.BeginAuthorityFacts{Prepared: current.Execution != nil, Artifact: current.RemoteArtifact != nil, ExpectedGeneration: expectedGeneration}
		if current.Execution != nil {
			facts.Pending = current.Execution.Pending != nil
			facts.CurrentGeneration = current.Execution.Lease.Generation
		}
		if err := publicationdomain.ValidateBeginAuthority(facts); err != nil {
			return err
		}
		payload.Generation = current.Execution.Lease.Generation
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		current.Execution.Pending = &issueops.ExternalIntent{OperationID: operationID, Kind: externalIntentRemotePR, Marker: marker, StartedAt: executionNow(now)}
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []port.RecordMutation{{
			Bucket: externalIntentBucket, ID: operationID, Data: data, RequireAbsent: true,
		}})
		return err
	})
	return persisted, payload, err
}

func recordRemotePullRequestFailure(stateRoot, id, operationID, invocation string, retryCount int, knownURL string, cause error, now func() time.Time) error {
	return withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		record, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		if record.Execution == nil || record.Execution.Pending == nil || record.Execution.Pending.OperationID != operationID {
			return fmt.Errorf("external intent changed before failure receipt")
		}
		payload, err := readExternalRemotePRPayload(stateRoot, operationID)
		if err != nil {
			return err
		}
		payload.InvocationState, payload.RetryCount = invocation, retryCount
		if strings.TrimSpace(knownURL) != "" {
			payload.KnownURL = strings.TrimSpace(knownURL)
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		message := boundedExecutionRemoteDiagnostic(cause)
		record.Execution.Failure = &issueops.ExecutionFailure{OperationID: operationID, Code: "external_operation_ambiguous", Message: message, At: executionNow(now)}
		_, err = persistExecutionTransitionWithMutations(stateRoot, record, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: operationID, Data: data}})
		return err
	})
}

func finishRemotePullRequestIntent(stateRoot, id string, payload externalRemotePRPayload, url string, enforceOriginalGeneration bool, now func() time.Time) (issueops.IssueOpsRecord, error) {
	var persisted issueops.IssueOpsRecord
	err := withIssueOpsLock(context.Background(), stateRoot, id, func(context.Context) error {
		current, err := ReadIssueOps(stateRoot, id)
		if err != nil {
			return err
		}
		facts := publicationdomain.ReceiptAuthorityFacts{Prepared: current.Execution != nil, ExpectedOperationID: payload.OperationID, ExpectedGeneration: payload.Generation}
		if current.Execution != nil {
			facts.Pending = current.Execution.Pending != nil
			if current.Execution.Pending != nil {
				facts.PendingOperationID = current.Execution.Pending.OperationID
			}
			lease := current.Execution.Lease
			facts.Generation, facts.LeaseStatus = lease.Generation, string(lease.Status)
			facts.ExpectedHost, facts.ExpectedSessionID, facts.ExpectedAgentID = payload.Request.Host, payload.Request.SessionID, payload.Request.AgentID
			if enforceOriginalGeneration && facts.Pending && facts.PendingOperationID == payload.OperationID {
				facts.CWDMatches = samePath(payload.Request.CWD, current.Execution.Workspace.Root)
			}
			if lease.Holder != nil {
				facts.HolderPresent = true
				facts.HolderHost, facts.HolderSessionID, facts.HolderAgentID = lease.Holder.Host, lease.Holder.SessionID, lease.Holder.AgentID
			}
		}
		if err := publicationdomain.ValidateReceiptAuthority(facts, enforceOriginalGeneration); err != nil {
			return err
		}
		stored, err := readExternalRemotePRPayload(stateRoot, payload.OperationID)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(stored, payload) {
			return fmt.Errorf("external intent payload changed before remote receipt CAS")
		}
		artifact, err := artifactverify.Projection(current, issueops.IssueOpsRemoteArtifactVerificationRequest{
			Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url),
			Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch,
		})
		if err != nil {
			return err
		}
		artifact.VerifiedAt = executionNow(now)
		current.RemoteArtifact = &artifact
		current.Execution.Pending = nil
		current.Execution.Failure = nil
		persisted, err = persistExecutionTransitionWithMutations(stateRoot, current, nil, []port.RecordMutation{{Bucket: externalIntentBucket, ID: payload.OperationID, Delete: true}})
		return err
	})
	return persisted, err
}

func readExternalRemotePRPayload(stateRoot, operationID string) (externalRemotePRPayload, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return externalRemotePRPayload{}, err
	}
	if !ok {
		return externalRemotePRPayload{}, fmt.Errorf("external intent payload is missing")
	}
	var payload externalRemotePRPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return externalRemotePRPayload{}, fmt.Errorf("decode external intent payload: %w", err)
	}
	if payload.SchemaVersion != issueops.IssueOpsSchemaVersion || payload.OperationID != operationID || payload.Generation == 0 || payload.Provider == "" || payload.Kind == "" {
		return externalRemotePRPayload{}, fmt.Errorf("external intent payload is invalid")
	}
	return payload, nil
}

func validateRemotePullRequestCandidate(record issueops.IssueOpsRecord, payload externalRemotePRPayload, candidate port.IssueProviderReconcilePullRequestCandidate) error {
	request := publicationcontract.ProviderCreateRequest{
		ProjectKey: payload.Request.ProjectKey, Title: payload.Request.Title, Body: payload.Request.Body,
		HeadBranch: payload.Request.HeadBranch, BaseBranch: payload.Request.BaseBranch,
		ExpectedHeadSHA: payload.Request.ExpectedHeadSHA, Labels: payload.Request.Labels,
		Assignees: payload.Request.Assignees, Draft: payload.Request.Draft,
	}
	observed := publicationcontract.Candidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256, Labels: candidate.Labels,
		Assignees: candidate.Assignees, Draft: candidate.Draft, State: candidate.State,
	}
	if err := publicationdomain.ValidateCandidate(request, observed, payload.KnownURL); err != nil {
		return err
	}
	if err := remote.ValidateArtifactURL(candidate.URL, payload.Provider, payload.Kind); err != nil {
		return err
	}
	codeProjectKey := ""
	if record.BranchPrepare != nil {
		codeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	return remote.ValidateArtifactMatchesProject(
		remote.EffectiveProjectKey(codeProjectKey, record.IssueURL, payload.Provider),
		candidate.URL, payload.Provider, payload.Kind)
}

func verifyRemotePullRequestResult(record issueops.IssueOpsRecord, payload externalRemotePRPayload, url string, verify RemoteArtifactVerifyFunc) error {
	req := issueops.IssueOpsRemoteArtifactVerificationRequest{
		Provider: payload.Provider, Kind: payload.Kind, URL: strings.TrimSpace(url),
		Labels: payload.Request.Labels, Assignees: payload.Request.Assignees, TargetBranch: payload.Request.BaseBranch,
	}
	if _, err := artifactverify.Projection(record, req); err != nil {
		return err
	}
	if verify != nil {
		return verify(req)
	}
	return nil
}

func validExecutionHead(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func boundedExecutionRemoteDiagnostic(err error) string {
	if err == nil {
		return "external operation failed"
	}
	message := strings.TrimSpace(policy.RedactDiagnostic(err.Error()))
	if len(message) > 4096 {
		message = message[:4096]
	}
	return message
}
