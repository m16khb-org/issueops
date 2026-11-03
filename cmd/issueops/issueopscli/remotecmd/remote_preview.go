package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"issueops/cmd/issueops/jsonout"
)

// previewView is what a preview-then-confirm remote metadata command exposes
// from its result so the shared runner can report it.
type previewView struct {
	// targetURL is empty when the operation failed before reaching a target.
	targetURL string
	// nextCommand points at the result's NextCommand field.
	nextCommand *string
	// done and ok select doneLine, else previewLine, as the human text.
	done        bool
	ok          bool
	doneLine    string
	previewLine string
}

// previewSpec describes one preview-then-confirm remote metadata command
// (assign, label, milestone, retitle). R is its result type and V the type of
// its one value flag.
type previewSpec[R, V any] struct {
	subcommand   string
	onUsage      string
	confirmUsage string
	bindValue    func(fs *flag.FlagSet) func() V
	operation    func(ctx context.Context, stateRoot, id, on string, value V, confirm bool) (R, error)
	view         func(result *R) previewView
}

// listPreviewView builds the view of a command that adds a list of values:
// doneFormat takes the target URL and the joined resulting values, previewFormat
// the joined values to add and the target URL.
func listPreviewView(targetURL string, nextCommand *string, done, ok bool, doneFormat, previewFormat string, resulting, add []string) previewView {
	return previewView{
		targetURL,
		nextCommand,
		done,
		ok,
		fmt.Sprintf(doneFormat, targetURL, strings.Join(resulting, ", ")),
		fmt.Sprintf(previewFormat, strings.Join(add, ", "), targetURL),
	}
}

// valuePreviewView builds the view of a command that sets one value: doneFormat
// takes the target URL and the read-back value.
func valuePreviewView(targetURL string, nextCommand *string, done, ok bool, doneFormat, readBack, previewLine string) previewView {
	return previewView{
		targetURL,
		nextCommand,
		done,
		ok,
		fmt.Sprintf(doneFormat, targetURL, readBack),
		previewLine,
	}
}

func stringValueFlag(name, usage string) func(fs *flag.FlagSet) func() string {
	return func(fs *flag.FlagSet) func() string {
		value := fs.String(name, "", usage)
		return func() string { return *value }
	}
}

func repeatedValueFlag(name, usage string) func(fs *flag.FlagSet) func() []string {
	return func(fs *flag.FlagSet) func() []string {
		var values repeatedFlag
		fs.Var(&values, name, usage)
		return func() []string { return values }
	}
}

// runPreviewCommand parses the command's flags, runs the operation, and
// reports: a bare error when no target was reached, otherwise the confirm hint
// (preview only), then JSON or the human lines plus the next command.
func runPreviewCommand[R, V any](command Command, args []string, deps Deps, spec previewSpec[R, V]) error {
	fs := flag.NewFlagSet("issueops remote "+spec.subcommand, flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", spec.onUsage)
	value := spec.bindValue(fs)
	confirm := fs.Bool("confirm", false, spec.confirmUsage)
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := spec.operation(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, value(), *confirm)
	view := spec.view(&result)
	if view.targetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		*view.nextCommand = confirmCommand(spec.subcommand, args)
	}
	if *jsonOut {
		if err != nil {
			if printErr := deps.printJSON(jsonout.WithError(result, err)); printErr != nil {
				return printErr
			}
			return jsonout.Reported(err)
		}
		return deps.printJSON(result)
	}
	switch {
	case view.done:
		fmt.Println(view.doneLine)
	case view.ok:
		fmt.Println(view.previewLine)
	}
	if *view.nextCommand != "" {
		fmt.Println("next:", *view.nextCommand)
	}
	return err
}
