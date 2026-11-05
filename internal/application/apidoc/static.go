package apidoc

import (
	"errors"
	"fmt"
	"strings"

	contract "issueops/internal/contract/apidoc"
	domain "issueops/internal/domain/apidoc"
)

// ErrStaticGateFailed is returned by StaticService.Check when any API
// documentation violation is found.
var ErrStaticGateFailed = errors.New("api documentation static check gate failed")

// StaticOptions selects the repository and files the static check covers;
// All falls back to every tracked file instead of the staged ones.
type StaticOptions struct {
	Repo  string
	Files []string
	All   bool
	JSON  bool
}

// StaticEffects injects the file listing, API-documentation mode, and file
// reading side effects the static check needs.
type StaticEffects struct {
	NormalizeFiles func(repo string, files []string) []string
	TrackedFiles   func(repo string) []string
	StagedFiles    func(repo string) []string
	Mode           func(repo string) string
	ReadFile       func(repo, file string) (string, error)
}

// StaticService checks TypeScript controllers and DTOs for missing Swagger
// documentation.
type StaticService struct{ Effects StaticEffects }

// Check reports violations for the candidate files. It skips with a passing
// result when no candidates exist or the repository uses contract-tests
// documentation, and returns ErrStaticGateFailed when violations remain.
func (service StaticService) Check(options StaticOptions) (contract.StaticResult, error) {
	files, summary := service.candidateFiles(options)
	if len(files) == 0 {
		return contract.StaticResult{OK: true, Summary: summary, Files: []string{}, Violations: []contract.Violation{}, Skipped: true, Reason: "no_api_doc_candidate_files"}, nil
	}
	if service.Effects.Mode(options.Repo) == "contract-tests" {
		return contract.StaticResult{
			OK: true, Summary: "Swagger decorator checks skipped; repository uses contract-tests API documentation.",
			Files: files, Violations: []contract.Violation{}, Skipped: true, Reason: "contract_tests_mode",
		}, nil
	}
	violations, err := service.collectViolations(options.Repo, files)
	if err != nil {
		return contract.StaticResult{OK: false, Summary: err.Error(), Files: files}, err
	}
	domain.SortViolations(violations)
	if violations == nil {
		violations = []contract.Violation{}
	}
	result := contract.StaticResult{OK: len(violations) == 0, Files: files, Violations: violations}
	if result.OK {
		result.Summary = "API documentation static check passed."
		return result, nil
	}
	result.Summary = fmt.Sprintf("API documentation static check found %d violation(s).", len(violations))
	return result, ErrStaticGateFailed
}

// candidateFiles resolves the files to check and the summary reported when
// none of them exist.
func (service StaticService) candidateFiles(options StaticOptions) ([]string, string) {
	files := service.Effects.NormalizeFiles(options.Repo, options.Files)
	if len(files) == 0 && options.All {
		files = service.Effects.TrackedFiles(options.Repo)
	}
	summary := "No API documentation candidate files."
	if len(files) == 0 && !options.All {
		files = service.Effects.StagedFiles(options.Repo)
		// QA: run before the commit, the staged default saw nothing and the
		// skip read as a pass while the changed controller went unchecked.
		if len(options.Files) == 0 {
			summary = "No staged API documentation candidate files: the default scope is the staged diff; before a commit, pass the changed files as arguments (issueops api-doc check --json FILE...)."
		}
	}
	return files, summary
}

// collectViolations reads each TypeScript file and gathers its controller and
// DTO violations in file order.
func (service StaticService) collectViolations(repo string, files []string) ([]contract.Violation, error) {
	var violations []contract.Violation
	for _, file := range files {
		if !strings.HasSuffix(file, ".ts") {
			continue
		}
		text, err := service.Effects.ReadFile(repo, file)
		if err != nil {
			return nil, err
		}
		controller, dto := domain.StaticKinds(file)
		if controller {
			violations = append(violations, domain.CheckNestController(file, text)...)
		}
		if dto {
			violations = append(violations, domain.CheckNestDTO(file, text)...)
		}
	}
	return violations, nil
}
