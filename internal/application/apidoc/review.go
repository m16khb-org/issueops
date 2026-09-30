package apidoc

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	contract "issueops/internal/contract/apidoc"
	domain "issueops/internal/domain/apidoc"
)

var (
	ErrReviewGateFailed     = errors.New("api documentation review gate failed")
	ErrReviewResultRequired = errors.New("api documentation host-agent review result required")
)

type ReviewOptions struct {
	Repo       string
	Files      []string
	All        bool
	DiffFile   string
	PromptFile string
	ResultFile string
	JSON       bool
}

type ReviewEffects struct {
	NormalizeFiles func(string, []string) []string
	TrackedFiles   func(string) []string
	StagedFiles    func(string) []string
	Input          func(string, []string, string, bool) (string, error)
	ExtraPrompt    func(ReviewOptions) (string, error)
	Evidence       func(string, []string) string
	BuildPrompt    func([]string, string, string, string) string
	Schema         func() map[string]any
	ReadResult     func(string, string) (string, []byte, error)
}

type ReviewService struct{ Effects ReviewEffects }

func (service ReviewService) Review(options ReviewOptions) (contract.ReviewResult, error) {
	files := service.Effects.NormalizeFiles(options.Repo, options.Files)
	if len(files) == 0 && options.All && options.DiffFile == "" {
		files = service.Effects.TrackedFiles(options.Repo)
	}
	if len(files) == 0 && !options.All && options.DiffFile == "" {
		files = service.Effects.StagedFiles(options.Repo)
	}
	if len(files) == 0 && options.DiffFile == "" {
		summary, reason := "No staged API documentation candidate files.", "no_api_doc_candidate_files"
		if options.All {
			summary, reason = "No tracked API documentation candidate files.", "no_tracked_api_doc_candidate_files"
		}
		return contract.ReviewResult{OK: true, Verdict: "pass", Summary: summary, Findings: []contract.ReviewFinding{}, Files: []string{}, Skipped: true, Reason: reason}, nil
	}
	diff, err := service.Effects.Input(options.Repo, files, options.DiffFile, options.All)
	if err != nil {
		return contract.ReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files}, err
	}
	if strings.TrimSpace(diff) == "" {
		summary := "No staged API documentation diff."
		if options.All {
			summary = "No API documentation content."
		}
		return contract.ReviewResult{OK: true, Verdict: "pass", Summary: summary, Findings: []contract.ReviewFinding{}, Files: files, Skipped: true, Reason: "empty_diff"}, nil
	}
	extraPrompt, err := service.Effects.ExtraPrompt(options)
	if err != nil {
		return contract.ReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files}, err
	}
	evidence := ""
	if options.ResultFile == "" {
		evidence = service.Effects.Evidence(options.Repo, files)
	}
	prompt := service.Effects.BuildPrompt(files, diff, extraPrompt, evidence)
	schema := service.Effects.Schema()
	if options.ResultFile == "" {
		return contract.ReviewResult{
			OK: false, Verdict: "pending", Summary: "Host-agent API documentation review result is required; run the prompt and pass --result <file>.",
			Findings: []contract.ReviewFinding{}, Files: files, Reason: "host_agent_result_required", Prompt: prompt, Schema: schema,
		}, ErrReviewResultRequired
	}
	resultPath, data, err := service.Effects.ReadResult(options.Repo, options.ResultFile)
	if err != nil {
		return contract.ReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, ResultFile: resultPath}, err
	}
	var result contract.ReviewResult
	if err := json.Unmarshal(data, &result); err != nil {
		return contract.ReviewResult{OK: false, Verdict: "fail", Summary: err.Error(), Files: files, ResultFile: resultPath}, err
	}
	if !domain.ValidReviewVerdict(result.Verdict) {
		return contract.ReviewResult{OK: false, Verdict: "fail", Summary: fmt.Sprintf("review result verdict must be pass or fail, got %q", result.Verdict), Files: files, ResultFile: resultPath}, fmt.Errorf("invalid API doc review result verdict %q", result.Verdict)
	}
	result.Files, result.ResultFile, result.OK = files, resultPath, result.Verdict == "pass"
	if !result.OK {
		return result, ErrReviewGateFailed
	}
	return result, nil
}
