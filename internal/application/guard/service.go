package guard

import (
	guardcontract "issueops/internal/contract/guard"
	guarddomain "issueops/internal/domain/guard"
)

type Source interface {
	ResolveRoot(string) string
	TargetFiles(string, guardcontract.GuardCheckRequest) []string
	ExistingSymbols(string, []string) map[string][]string
	ReadFile(root, rel string, staged bool) (string, bool)
}

type Service struct{ Source Source }

func (service Service) Check(request guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult {
	root := service.Source.ResolveRoot(request.RepoRoot)
	files := service.Source.TargetFiles(root, request)
	existingSymbols := service.Source.ExistingSymbols(root, files)
	observed := make([]guarddomain.FileObservation, 0, len(files))
	for _, rel := range files {
		if guarddomain.SecretLikePath(rel) {
			observed = append(observed, guarddomain.FileObservation{Path: rel})
			continue
		}
		content, ok := service.Source.ReadFile(root, rel, request.Staged)
		observed = append(observed, guarddomain.FileObservation{Path: rel, Content: content, Read: ok})
	}
	analysis := guarddomain.Analyze(observed, existingSymbols)
	return guardcontract.GuardCheckResult{
		OK: analysis.OK, RepoRoot: root, Mode: guarddomain.Mode(request), CheckedFiles: files,
		Findings: analysis.Findings, Summary: analysis.Summary, Warnings: []string{},
	}
}
