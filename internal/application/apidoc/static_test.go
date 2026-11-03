package apidoc

import (
	"errors"
	"testing"
)

func TestStaticCheckSkipsWithoutCandidateFiles(t *testing.T) {
	service := StaticService{Effects: StaticEffects{
		NormalizeFiles: func(string, []string) []string { return nil },
		StagedFiles:    func(string) []string { return nil },
	}}
	result, err := service.Check(StaticOptions{Repo: "/repo"})
	if err != nil || !result.OK || !result.Skipped || result.Reason != "no_api_doc_candidate_files" || result.Violations == nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStaticCheckExplicitFilesTakePrecedenceOverTracked(t *testing.T) {
	service := StaticService{Effects: StaticEffects{
		NormalizeFiles: func(string, []string) []string { return []string{"given.yaml"} },
		TrackedFiles:   func(string) []string { t.Fatal("tracked files consulted despite explicit files"); return nil },
		StagedFiles:    func(string) []string { t.Fatal("staged files consulted despite explicit files"); return nil },
		Mode:           func(string) string { return "" },
	}}
	result, err := service.Check(StaticOptions{All: true, Files: []string{"given.yaml"}})
	if err != nil || !result.OK || len(result.Files) != 1 || result.Files[0] != "given.yaml" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStaticCheckAllUsesTrackedFilesAndReportsReadFailure(t *testing.T) {
	readErr := errors.New("read failed")
	service := StaticService{Effects: StaticEffects{
		NormalizeFiles: func(string, []string) []string { return nil },
		TrackedFiles:   func(string) []string { return []string{"README.md", "src/user.controller.ts"} },
		StagedFiles:    func(string) []string { t.Fatal("staged files consulted despite --all"); return nil },
		Mode:           func(string) string { return "" },
		ReadFile:       func(string, string) (string, error) { return "", readErr },
	}}
	result, err := service.Check(StaticOptions{All: true})
	if !errors.Is(err, readErr) || result.OK || result.Summary != "read failed" || len(result.Files) != 2 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestStaticCheckReportsControllerViolationsAndSkipsNonTypeScript(t *testing.T) {
	service := StaticService{Effects: StaticEffects{
		NormalizeFiles: func(string, []string) []string { return []string{"docs/route.md", "src/user.controller.ts"} },
		Mode:           func(string) string { return "" },
		ReadFile: func(_, file string) (string, error) {
			if file != "src/user.controller.ts" {
				t.Fatalf("read non-TypeScript file %s", file)
			}
			return "@Controller('users')\nexport class UserController {\n  @Get()\n  list() {\n    return [];\n  }\n}\n", nil
		},
	}}
	result, err := service.Check(StaticOptions{Files: []string{"x"}})
	if !errors.Is(err, ErrStaticGateFailed) || result.OK || len(result.Violations) == 0 || result.Violations[0].File != "src/user.controller.ts" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
