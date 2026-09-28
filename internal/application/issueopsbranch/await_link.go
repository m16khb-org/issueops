package issueopsbranch

import (
	"context"
	"fmt"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	linkedbranch "issueops/internal/domain/issueopslinkedbranch"
)

// LinkAwaiter only observes the coordinator's sealed branch identity. It never
// creates a remote branch or persists verification evidence.
type LinkAwaiter struct {
	Load                  func(string) (issueopscontract.IssueOpsRecord, error)
	RemoteRef             func(context.Context, string, string) (string, error)
	ObserveLinkedBranches func(context.Context, string) (linkedbranch.Observation, error)
	Sleep                 func(context.Context, time.Duration) error
	Now                   func() time.Time
}

func (s LinkAwaiter) Await(ctx context.Context, req issueopscontract.AwaitBranchLinkRequest) (issueopscontract.AwaitBranchLinkResult, error) {
	record, err := s.Load(req.ID)
	if err != nil {
		return issueopscontract.AwaitBranchLinkResult{OK: false, ID: req.ID}, err
	}
	result := issueopscontract.AwaitBranchLinkResult{OK: true, ID: record.ID}

	timeout, err := domain.AwaitBranchLinkTimeout(req.Timeout)
	if err != nil {
		result.OK = false
		return result, err
	}
	result.TimeoutSeconds = int(timeout / time.Second)

	if missing := domain.AwaitBranchLinkGates(record, s.ObserveLinkedBranches != nil); len(missing) > 0 {
		result.OK, result.Missing = false, missing
		return result, fmt.Errorf("branch await-link is not ready: %s", strings.Join(missing, ", "))
	}
	prepare := record.BranchPrepare
	result.IssueURL, result.Branch, result.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA

	// 이미 기록돼 있으면 기다리지 않는다. 멱등 성공이다.
	if prepare.LinkVerified {
		result.AlreadyVerified, result.Linked = true, true
		return result, nil
	}

	deadline := s.Now().Add(timeout)
	for {
		state, node, reason := s.observe(ctx, record)
		result.Attempts++
		result.State, result.StateReason = string(state), reason
		if state == linkedbranch.StateHealthy {
			result.Linked, result.LinkedBranchID, result.ObservedOID = true, node.ID, node.RefOID
			result.NextCommand = "record the link with the sealed verify_branch_link command from the owner packet for " + record.ID
			return result, nil
		}
		// mismatched는 기다린다고 낫지 않는다. 링크는 있는데 봉인값과 다르므로
		// 사람이 봐야 한다 — 기다리면 진단만 늦어진다.
		if state == linkedbranch.StateMismatched {
			result.OK = false
			return result, fmt.Errorf("linked branch does not match the sealed identity: %s", reason)
		}
		if !s.Now().Before(deadline) {
			result.OK, result.TimedOut = false, true
			return result, fmt.Errorf(
				"the coordinator's linked branch for %s did not appear within %s (last state: %s); "+
					"the coordinator must create it at the sealed base %s",
				prepare.Branch, timeout, state, prepare.BaseSHA)
		}
		if err := s.Sleep(ctx, domain.AwaitBranchLinkInterval); err != nil {
			result.OK = false
			return result, err
		}
	}
}

// observe는 issue 링크와 원격 ref를 같은 시점에 읽어 분류한다.
// #306의 분류기를 그대로 쓴다 — "정상 링크"의 정의가 두 곳에서 갈리면 한쪽이
// 통과시킨 상태를 다른 쪽이 거부하는 교착이 다시 생긴다.
func (s LinkAwaiter) observe(ctx context.Context, record issueopscontract.IssueOpsRecord) (linkedbranch.State, linkedbranch.Node, string) {
	prepare := record.BranchPrepare
	observation, err := s.ObserveLinkedBranches(ctx, prepare.IssueURL)
	if err != nil {
		// 관측 실패는 부재가 아니다. 다음 주기에 다시 읽는다.
		return linkedbranch.StateAmbiguous, linkedbranch.Node{}, "linked branch readback failed: " + err.Error()
	}
	observation.IssueURL, observation.RequestedBranch, observation.SealedBase = prepare.IssueURL, prepare.Branch, prepare.BaseSHA
	observation.RemoteOID, err = s.RemoteRef(ctx, record.Repo, prepare.Branch)
	if err != nil {
		return linkedbranch.StateAmbiguous, linkedbranch.Node{}, err.Error()
	}
	return linkedbranch.Classify(observation)
}
