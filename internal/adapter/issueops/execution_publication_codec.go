package issueops

import (
	"issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

func publicationActor(actor contract.Actor) issueops.NativeActor {
	result := issueops.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &issueops.NativeProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	if actor.ProcessAncestry != nil {
		result.ProcessAncestry = make([]issueops.NativeProcessReceipt, len(actor.ProcessAncestry))
		for index, receipt := range actor.ProcessAncestry {
			result.ProcessAncestry[index] = issueops.NativeProcessReceipt{
				PID: receipt.PID, StartedAt: receipt.StartedAt, Executable: receipt.Executable,
			}
		}
	}
	return result
}

func publicationRequest(request port.IssueProviderCreatePullRequestRequest) contract.ProviderCreateRequest {
	return contract.ProviderCreateRequest{
		Repo: request.Repo, ProjectKey: request.ProjectKey, Title: request.Title, Body: request.Body,
		HeadBranch: request.HeadBranch, BaseBranch: request.BaseBranch,
		Labels: clonePublicationStrings(request.Labels), Assignees: clonePublicationStrings(request.Assignees),
		Draft: request.Draft, ExpectedHeadSHA: request.ExpectedHeadSHA, Confirm: request.Confirm,
		Host: request.Host, SessionID: request.SessionID, AgentID: request.AgentID, CWD: request.CWD,
	}
}

func portPublicationCandidate(candidate contract.Candidate) port.IssueProviderReconcilePullRequestCandidate {
	return port.IssueProviderReconcilePullRequestCandidate{
		URL: candidate.URL, ProjectKey: candidate.ProjectKey, SourceProjectKey: candidate.SourceProjectKey,
		HeadBranch: candidate.HeadBranch, BaseBranch: candidate.BaseBranch, HeadSHA: candidate.HeadSHA,
		Title: candidate.Title, BodySHA256: candidate.BodySHA256,
		Labels: clonePublicationStrings(candidate.Labels), Assignees: clonePublicationStrings(candidate.Assignees),
		Draft: candidate.Draft, State: candidate.State,
	}
}

func clonePublicationStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
