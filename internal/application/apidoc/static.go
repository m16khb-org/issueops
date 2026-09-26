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
	files := service.Effects.NormalizeFiles(options.Repo, options.Files)
	if len(files) == 0 && options.All {
		files = service.Effects.TrackedFiles(options.Repo)
	}
	if len(files) == 0 && !options.All {
		files = service.Effects.StagedFiles(options.Repo)
	}
	if len(files) == 0 {
		return contract.StaticResult{OK: true, Summary: "No API documentation candidate files.", Files: []string{}, Violations: []contract.Violation{}, Skipped: true, Reason: "no_api_doc_candidate_files"}, nil
	}
	if service.Effects.Mode(options.Repo) == "contract-tests" {
		return contract.StaticResult{
			OK: true, Summary: "Swagger decorator checks skipped; repository uses contract-tests API documentation.",
			Files: files, Violations: []contract.Violation{}, Skipped: true, Reason: "contract_tests_mode",
		}, nil
	}
	var violations []contract.Violation
	for _, file := range files {
		if !strings.HasSuffix(file, ".ts") {
			continue
		}
		text, err := service.Effects.ReadFile(options.Repo, file)
		if err != nil {
			return contract.StaticResult{OK: false, Summary: err.Error(), Files: files}, err
		}
		controller, dto := domain.StaticKinds(file)
		if controller {
			violations = append(violations, domain.CheckNestController(file, text)...)
		}
		if dto {
			violations = append(violations, domain.CheckNestDTO(file, text)...)
		}
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
