package issueopsremote

import (
	"fmt"

	"issueops/internal/domain/judgement"
)

func renderIssueOpsRemoteLLMJudgePrompt(req LLMJudgeRequest) (string, error) {
	prompt, err := buildIssueOpsRemoteLLMJudgePrompt(req.Request)
	if err != nil {
		return "", err
	}
	return prompt, nil
}

// RenderIssueOpsRemoteJudgePrompt builds the host-agent judge prompt for req and marks it as a read-only background_join that must finish before remote_artifact_write.
func RenderIssueOpsRemoteJudgePrompt(req LLMJudgeRequest) (JudgePromptResult, error) {
	prompt, err := renderIssueOpsRemoteLLMJudgePrompt(req)
	if err != nil {
		return JudgePromptResult{}, err
	}
	return JudgePromptResult{
		OK:             true,
		ExecutionClass: "background_join",
		ReadOnly:       true,
		JoinBefore:     "remote_artifact_write",
		Prompt:         prompt,
	}, nil
}

// DecodeIssueOpsRemoteJudgeJSON strictly decodes the host agent's scoring JSON, which must be ok, background_join, read_only, and joined before remote_artifact_write, and returns it normalized (no nil slices).
func DecodeIssueOpsRemoteJudgeJSON(out []byte) (ScoringResult, error) {
	result, err := decodeStrictIssueOpsRemoteScoringResult(out)
	if err != nil {
		return ScoringResult{}, err
	}
	return normalizeIssueOpsRemoteScoringResult(result), nil
}

func decodeStrictIssueOpsRemoteScoringResult(out []byte) (ScoringResult, error) {
	var result ScoringResult
	if err := judgement.DecodeStructuredJSONObject("issueops remote host-agent scoring", out, &result); err != nil {
		return ScoringResult{}, err
	}
	if !result.OK {
		return ScoringResult{}, fmt.Errorf("issueops remote host-agent scoring output not ok")
	}
	if result.ExecutionClass != "background_join" {
		return ScoringResult{}, fmt.Errorf("issueops remote host-agent scoring execution_class must be background_join")
	}
	if !result.ReadOnly {
		return ScoringResult{}, fmt.Errorf("issueops remote host-agent scoring read_only must be true")
	}
	if result.JoinBefore != "remote_artifact_write" {
		return ScoringResult{}, fmt.Errorf("issueops remote host-agent scoring join_before must be remote_artifact_write")
	}
	return result, nil
}
