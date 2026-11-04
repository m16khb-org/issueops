package projectdocs

import (
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

// RevisionEffects is the file and clock access a revision needs.
type RevisionEffects interface {
	Read(path string) (string, bool, error)
	Write(path, content string) error
	Now() time.Time
}

func revise(request projectdocscontract.ReviseRequest, root, rel, path string, effects RevisionEffects) (projectdocscontract.ReviseResult, error) {
	input := projectdocdomain.RevisionInput{
		Content: request.Content, ExpectedSHA256: request.ExpectedSHA256, Summary: request.Summary,
		Evidence: request.Evidence, Confirm: request.Confirm,
	}
	if err := projectdocdomain.ValidateRevisionInput(input); err != nil {
		return projectdocscontract.ReviseResult{}, err
	}
	current, exists, err := effects.Read(path)
	if err != nil {
		return projectdocscontract.ReviseResult{}, err
	}
	plan, err := projectdocdomain.PlanRevision(input, rel, current, exists)
	if err != nil {
		return projectdocscontract.ReviseResult{}, err
	}
	if plan.Write {
		if err := effects.Write(path, plan.Content); err != nil {
			return projectdocscontract.ReviseResult{}, err
		}
	}
	return projectdocscontract.ReviseResult{
		OK: true, Kind: "project_docs_revise", RepoRoot: root, RelPath: rel, Path: path,
		Action: plan.Action, Confirmed: request.Confirm, DryRun: !request.Confirm,
		GeneratedAt: effects.Now().UTC().Format(time.RFC3339), CurrentSHA256: plan.CurrentSHA256,
		NextSHA256: plan.NextSHA256, Bytes: len([]byte(plan.Content)), Summary: plan.Summary,
		Evidence: plan.Evidence, Warnings: plan.Warnings,
	}, nil
}
