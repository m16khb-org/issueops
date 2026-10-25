package guard

import (
	"strings"

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
		Findings: analysis.Findings, Summary: analysis.Summary, Warnings: skippedFileWarnings(request.Files, files),
	}
}

// skippedFileWarnings names explicitly requested files guard did not read.
// Probe: `guard check -- leak.txt` answered ok with checked_files [] and no
// hint that the file was never scanned (.txt is outside guard's scope).
func skippedFileWarnings(requested, checked []string) []string {
	seen := map[string]bool{}
	for _, file := range checked {
		seen[file] = true
	}
	var skipped []string
	for _, file := range requested {
		if clean := strings.TrimSpace(file); clean != "" && !seen[clean] {
			skipped = append(skipped, clean)
		}
	}
	if len(skipped) == 0 {
		return []string{}
	}
	return []string{"not checked: " + strings.Join(skipped, ", ") + " (guard reads repo-relative source, config, and docs files plus secret-like paths; other extensions, absolute paths, and paths outside the repo are skipped)"}
}

func (service Service) CheckForCommand(request guardcontract.GuardCheckRequest) (guardcontract.GuardCheckResult, error) {
	result := service.Check(request)
	return result, guarddomain.BlockingError(result)
}
