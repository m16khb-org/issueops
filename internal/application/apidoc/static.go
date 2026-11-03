package apidoc

import (
	"errors"
	"fmt"
	"strings"

	contract "issueops/internal/contract/apidoc"
	domain "issueops/internal/domain/apidoc"
)

var ErrStaticGateFailed = errors.New("api documentation static check gate failed")

type StaticOptions struct {
	Repo  string
	Files []string
	All   bool
	JSON  bool
}

type StaticEffects struct {
	NormalizeFiles func(repo string, files []string) []string
	TrackedFiles   func(repo string) []string
	StagedFiles    func(repo string) []string
	Mode           func(repo string) string
	ReadFile       func(repo, file string) (string, error)
}

type StaticService struct{ Effects StaticEffects }

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
