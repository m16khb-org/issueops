package apidoc

import (
	"errors"
	"reflect"
	"strings"
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

func reviewCandidateService(t *testing.T, tracked, staged []string, seen *[]string) ReviewService {
	t.Helper()
	return ReviewService{Effects: ReviewEffects{
		NormalizeFiles: func(_ string, files []string) []string { return files },
		TrackedFiles:   func(string) []string { *seen = append(*seen, "tracked"); return tracked },
		StagedFiles:    func(string) []string { *seen = append(*seen, "staged"); return staged },
		Input: func(_ string, files []string, diff string, _ bool) (string, error) {
			*seen = append(*seen, "input:"+strings.Join(files, ",")+"|"+diff)
			return "diff", nil
		},
		ExtraPrompt: func(ReviewOptions) (string, error) { return "", nil },
		Evidence:    func(string, []string) string { return "" },
		BuildPrompt: func([]string, string, string, string) string { return "" },
		Schema:      func() map[string]any { return nil },
	}}
}

func TestReviewCandidateFileSelection(t *testing.T) {
	tests := []struct {
		name      string
		options   ReviewOptions
		wantFiles []string
		wantSeen  []string
	}{
		{"explicit files win over tracked when all", ReviewOptions{Files: []string{"given.yaml"}, All: true}, []string{"given.yaml"}, []string{"input:given.yaml|"}},
		{"explicit files win over staged", ReviewOptions{Files: []string{"given.yaml"}}, []string{"given.yaml"}, []string{"input:given.yaml|"}},
		{"all falls back to tracked", ReviewOptions{All: true}, []string{"tracked.yaml"}, []string{"tracked", "input:tracked.yaml|"}},
		{"default falls back to staged", ReviewOptions{}, []string{"staged.yaml"}, []string{"staged", "input:staged.yaml|"}},
		{"diff file with all skips tracked fallback", ReviewOptions{All: true, DiffFile: "d.diff"}, []string{}, []string{"input:|d.diff"}},
		{"diff file skips staged fallback", ReviewOptions{DiffFile: "d.diff"}, []string{}, []string{"input:|d.diff"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var seen []string
			service := reviewCandidateService(t, []string{"tracked.yaml"}, []string{"staged.yaml"}, &seen)
			result, err := service.Review(test.options)
			if !errors.Is(err, ErrReviewResultRequired) {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if !reflect.DeepEqual(seen, test.wantSeen) {
				t.Fatalf("effects=%v want %v", seen, test.wantSeen)
			}
			if len(test.wantFiles) == 0 && len(result.Files) != 0 || len(test.wantFiles) > 0 && !reflect.DeepEqual(result.Files, test.wantFiles) {
				t.Fatalf("files=%v want %v", result.Files, test.wantFiles)
			}
		})
	}
}
