package goformat

import (
	"context"
	"time"

	verification "issueops/internal/adapter/verification"
	application "issueops/internal/application/selfverify"
	verifycontract "issueops/internal/contract/selfverify"
)

// Label is the self-verify step label. Command mirrors the CI "Format check
// (gofmt)" step verbatim so a failing step names exactly what CI runs.
const (
	Label   = "gofmt"
	Command = application.FormatCommand
)

type StepResult = verifycontract.StepResult

// Deps is the narrow process boundary so unit tests can replace git and gofmt
// without a repository or the Go toolchain.
type Deps struct {
	ListTrackedGoFiles func(ctx context.Context, root string) ([]string, error)
	ListUnformatted    func(ctx context.Context, root string, files []string) ([]string, error)
}

func (d Deps) withDefaults() Deps {
	if d.ListTrackedGoFiles == nil {
		d.ListTrackedGoFiles = verification.ListTrackedGoFiles
	}
	if d.ListUnformatted == nil {
		d.ListUnformatted = verification.ListUnformatted
	}
	return d
}

// Validate reports whether every git-tracked .go file under root is
// gofmt-clean. It is the local twin of the CI format gate: same file set
// (`git ls-files '*.go'`), same tool, same pass condition (empty `gofmt -l`
// output). `gofmt -l` exits 0 even when it lists files, so the verdict is
// output-based rather than exit-code-based.
func Validate(root string) StepResult {
	return ValidateWithDeps(root, Deps{})
}

func ValidateWithDeps(root string, deps Deps) StepResult {
	deps = deps.withDefaults()
	return application.ValidateFormat(root, application.FormatDeps{
		ListTrackedGoFiles: deps.ListTrackedGoFiles,
		ListUnformatted:    deps.ListUnformatted,
		Now:                time.Now,
	})
}
