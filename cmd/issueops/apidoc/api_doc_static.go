package apidoc

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
)

type apiDocStaticResult = contract.StaticResult
type apiDocStaticOptions = app.StaticOptions

func runAPIDocStaticCheck(args []string) error {
	fs := flag.NewFlagSet("api-doc static-check", flag.ContinueOnError)
	repo := fs.String("repo", "", "target git repository; defaults to current working directory")
	all := fs.Bool("all", false, "check all tracked API documentation candidate files instead of staged changes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	result, err := runAPIDocStaticCheckWithOptions(apiDocStaticOptions{Repo: ResolveTarget(*repo), Files: fs.Args(), All: *all, JSON: *jsonOut})
	if *jsonOut {
		_ = printJSON(result)
		return err
	}
	printAPIDocStaticCheck(result)
	return err
}

func runAPIDocStaticCheckWithOptions(options apiDocStaticOptions) (apiDocStaticResult, error) {
	return (app.StaticService{Effects: app.StaticEffects{
		NormalizeFiles: normalizeAPIDocFiles,
		TrackedFiles:   trackedAPIDocFiles,
		StagedFiles:    stagedAPIDocFiles,
		Mode:           apiDocMode,
		ReadFile: func(repo, file string) (string, error) {
			content, err := os.ReadFile(filepath.Join(repo, filepath.Clean(file)))
			return string(content), err
		},
	}}).Check(options)
}

func apiDocMode(repo string) string {
	content, err := os.ReadFile(filepath.Join(repo, ".issueops", "OPEN_API_SPEC.md"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return ""
	}
	for _, line := range lines[1:] {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			break
		}
		key, value, found := strings.Cut(trimmed, ":")
		if found && strings.TrimSpace(key) == "api_doc_mode" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
