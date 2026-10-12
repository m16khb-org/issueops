package apidoc

import "testing"

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
