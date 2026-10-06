package apidoc

import (
	"errors"
	app "issueops/internal/application/apidoc"
	contract "issueops/internal/contract/apidoc"
	"strings"
	"testing"
)

func TestAPIDocReviewRunnersKeepIndependentFiles(t *testing.T) {
	makeRunner := func(name string) func(app.ReviewOptions) (contract.ReviewResult, error) {
		service := testAPIDocService().Reviewer
		service.Effects.NormalizeFiles = func(_ string, files []string) []string { return files }
		service.Effects.Input = func(string, []string, string, bool) (string, error) { return name + "-input", nil }
		service.Effects.ExtraPrompt = func(app.ReviewOptions) (string, error) { return name + "-instructions", nil }
		service.Effects.Evidence = func(string, []string) string { return name + "-evidence" }
		return service.Review
	}
	a, b := makeRunner("first"), makeRunner("second")
	for _, tc := range []struct {
		name string
		run  func(app.ReviewOptions) (contract.ReviewResult, error)
	}{{"first", a}, {"second", b}, {"first", a}} {
		result, err := tc.run(app.ReviewOptions{Repo: "repo", Files: []string{"api/openapi.yaml"}})
		if !errors.Is(err, app.ErrReviewResultRequired) {
			t.Fatalf("review err=%v", err)
		}
		for _, suffix := range []string{"-input", "-instructions", "-evidence"} {
			if !strings.Contains(result.Prompt, tc.name+suffix) {
				t.Fatalf("%s runner used another instance: %s", tc.name, result.Prompt)
			}
		}
	}
}

func TestAPIDocCommandsKeepIndependentServices(t *testing.T) {
	makeCommand := func(name string) Command {
		service := testAPIDocService()
		normalize := func(repo string, _ []string) []string {
			if repo != name {
				t.Fatalf("%s service received %s", name, repo)
			}
			return []string{name + ".openapi.yaml"}
		}
		service.Static.Effects.NormalizeFiles = normalize
		service.Static.Effects.Mode = func(string) string { return "" }
		service.Reviewer.Effects.NormalizeFiles = normalize
		service.Reviewer.Effects.Input = func(string, []string, string, bool) (string, error) { return name + "-content", nil }
		service.Reviewer.Effects.ExtraPrompt = func(app.ReviewOptions) (string, error) { return "", nil }
		service.Reviewer.Effects.Evidence = func(string, []string) string { return "" }
		return Command{Service: service, ResolveTarget: func(string) string { return name }}
	}
	a, b := makeCommand("first"), makeCommand("second")
	for _, tc := range []struct {
		name    string
		command Command
	}{{"first", a}, {"second", b}, {"first", a}} {
		for _, operation := range []string{"static-check", "review", "check"} {
			output, err := captureAPIDocStdout(t, func() error { return tc.command.Run([]string{operation, "--json"}) })
			if operation == "static-check" && err != nil || operation != "static-check" && !errors.Is(err, app.ErrReviewResultRequired) {
				t.Fatalf("%s err=%v", operation, err)
			}
			if !strings.Contains(output, tc.name+".openapi.yaml") {
				t.Fatalf("%s %s output=%s", tc.name, operation, output)
			}
		}
	}
}
