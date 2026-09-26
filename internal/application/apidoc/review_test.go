package apidoc

import (
	"errors"
	"testing"
)

func TestReviewRequiresHostResultAndPreservesPrompt(t *testing.T) {
	service := ReviewService{Effects: ReviewEffects{
		NormalizeFiles: func(_ string, files []string) []string { return files },
		Input:          func(string, []string, string, bool) (string, error) { return "diff", nil },
		ExtraPrompt:    func(ReviewOptions) (string, error) { return "extra", nil },
		Evidence:       func(string, []string) string { return "evidence" },
		BuildPrompt:    func(_ []string, _, _, evidence string) string { return evidence },
		Schema:         func() map[string]any { return map[string]any{"type": "object"} },
	}}
	result, err := service.Review(ReviewOptions{Files: []string{"api/openapi.yaml"}})
	if !errors.Is(err, ErrReviewResultRequired) || result.Prompt != "evidence" || result.Verdict != "pending" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestReviewRejectsInvalidVerdict(t *testing.T) {
	service := ReviewService{Effects: ReviewEffects{
		NormalizeFiles: func(_ string, files []string) []string { return files },
		Input:          func(string, []string, string, bool) (string, error) { return "diff", nil },
		ExtraPrompt:    func(ReviewOptions) (string, error) { return "", nil },
		BuildPrompt:    func([]string, string, string, string) string { return "" },
		Schema:         func() map[string]any { return nil },
		ReadResult: func(string, string) (string, []byte, error) {
			return "/result.json", []byte(`{"verdict":"maybe"}`), nil
		},
	}}
	result, err := service.Review(ReviewOptions{Files: []string{"api/openapi.yaml"}, ResultFile: "result.json"})
	if err == nil || result.Verdict != "fail" || result.ResultFile != "/result.json" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
