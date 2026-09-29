package apidoc

import (
	"errors"
	"reflect"
	"testing"
)

func TestCombinedCheckStopsBeforeReviewOnStaticFailure(t *testing.T) {
	for _, readErr := range []error{nil, errors.New("unreadable")} {
		service := Service{
			Static: StaticService{Effects: StaticEffects{
				NormalizeFiles: func(string, []string) []string { return []string{"user.dto.ts"} },
				Mode:           func(string) string { return "" },
				ReadFile:       func(string, string) (string, error) { return "export class UserDto {\n name: string\n}", readErr },
			}},
			Reviewer: ReviewService{Effects: ReviewEffects{NormalizeFiles: func(string, []string) []string { t.Fatal("review ran after failed static check"); return nil }}},
		}
		result, err := service.Check(StaticOptions{Repo: "static-repo"}, ReviewOptions{Repo: "review-repo"})
		if err == nil || err.Error() != "api documentation static check failed" || result.OK || result.Static.OK || result.Reason != "static_check_failed" || !result.Review.OK || !result.Review.Skipped || result.Review.Findings == nil || !reflect.DeepEqual(result.Review.Files, result.Static.Files) {
			t.Fatalf("result=%+v, err=%v", result, err)
		}
	}
}

func TestCombinedCheckPassesReviewOptionsAfterStaticSuccess(t *testing.T) {
	var order []string
	service := Service{
		Static: StaticService{Effects: StaticEffects{
			NormalizeFiles: func(repo string, _ []string) []string {
				order = append(order, "static:"+repo)
				return []string{"api/openapi.yaml"}
			},
			Mode: func(string) string { return "" },
		}},
		Reviewer: ReviewService{Effects: ReviewEffects{
			NormalizeFiles: func(repo string, files []string) []string { order = append(order, "review:"+repo); return files },
			Input: func(repo string, files []string, diff string, all bool) (string, error) {
				if diff != "chosen.diff" || !all || files[0] != "chosen.yaml" {
					t.Fatal("review options lost")
				}
				return "content", nil
			},
			ExtraPrompt: func(o ReviewOptions) (string, error) {
				if o.PromptFile != "prompt" {
					t.Fatal("prompt option lost")
				}
				return "", nil
			},
			BuildPrompt: func([]string, string, string, string) string { return "prompt" }, Schema: func() map[string]any { return nil },
			ReadResult: func(repo, file string) (string, []byte, error) { return file, []byte(`{"verdict":"fail"}`), nil },
		}},
	}
	result, err := service.Check(StaticOptions{Repo: "static"}, ReviewOptions{Repo: "review", Files: []string{"chosen.yaml"}, DiffFile: "chosen.diff", PromptFile: "prompt", ResultFile: "result", All: true})
	if !errors.Is(err, ErrReviewGateFailed) || result.OK || !result.Static.OK || result.Review.Verdict != "fail" || result.Review.ResultFile != "result" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(order, []string{"static:static", "review:review"}) {
		t.Fatal(order)
	}
}
