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
