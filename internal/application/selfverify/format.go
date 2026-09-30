package selfverify

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	contract "issueops/internal/contract/selfverify"
	domain "issueops/internal/domain/selfverify"
)

const (
	FormatLabel        = "gofmt"
	FormatCommand      = "gofmt -l $(git ls-files '*.go')"
	formatTimeout      = 2 * time.Minute
	formatOutputBudget = 8 * 1024
)

type FormatDeps struct {
	ListTrackedGoFiles func(context.Context, string) ([]string, error)
	ListUnformatted    func(context.Context, string, []string) ([]string, error)
	Now                func() time.Time
}

func ValidateFormat(root string, deps FormatDeps) contract.StepResult {
	started := deps.Now()
	ctx, cancel := context.WithTimeout(context.Background(), formatTimeout)
	defer cancel()
	files, err := deps.ListTrackedGoFiles(ctx, root)
	if err != nil {
		return failedFormat(deps.Now().Sub(started).Milliseconds(), fmt.Errorf("list tracked .go files: %w", err))
	}
	if len(files) == 0 {
		return failedFormat(deps.Now().Sub(started).Milliseconds(), errors.New("no tracked .go files found; gofmt parity cannot be verified"))
	}
	unformatted, err := deps.ListUnformatted(ctx, root, files)
	if err != nil {
		return failedFormat(deps.Now().Sub(started).Milliseconds(), fmt.Errorf("gofmt -l: %w", err))
	}
	errs := []string{}
	if len(unformatted) > 0 {
		errs = append(errs, fmt.Sprintf("%d tracked .go file(s) are not gofmt-clean; run gofmt -w on: %s", len(unformatted), strings.Join(unformatted, " ")))
	}
	stdout := []string{fmt.Sprintf("checked %d tracked .go file(s)", len(files))}
	return domain.AssertionStepWithOutput(FormatLabel, deps.Now().Sub(started).Milliseconds(), errs, stdout, []string{FormatCommand}, formatOutputBudget)
}

func failedFormat(durationMS int64, err error) contract.StepResult {
	step := domain.FailedStep(FormatLabel, err)
	step.Command = FormatCommand
	step.DurationMS = durationMS
	return step
}
