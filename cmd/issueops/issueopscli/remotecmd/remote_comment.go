package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"issueops/cmd/issueops/jsonout"
	"issueops/internal/domain/shelltoken"
)

func (command Command) runRemoteComment(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote comment", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	on := fs.String("on", "", "comment target: issue or pr (a GitLab MR is pr)")
	body := fs.String("body", "", "comment text (Korean)")
	bodyFile := fs.String("body-file", "", "comment markdown file (Korean)")
	confirm := fs.Bool("confirm", false, "post the comment; without this, preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	text := *body
	if strings.TrimSpace(*bodyFile) != "" {
		if strings.TrimSpace(*body) != "" {
			return deps.printErrorResult(*jsonOut, fmt.Errorf("pass either --body or --body-file, not both"))
		}
		data, err := os.ReadFile(*bodyFile)
		if err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
		text = string(data)
	}
	result, err := command.Operations.CommentRemote(context.Background(), command.Operations.IssueOpsStateRoot(), *id, *on, text, *confirm)
	if result.TargetURL == "" {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err == nil && !*confirm {
		result.NextCommand = confirmCommand("comment", args)
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
	case result.Posted:
		fmt.Printf("commented on %s: %s\n", result.TargetURL, result.CommentURL)
	case result.OK:
		fmt.Printf("would comment on %s (%d characters); re-run with --confirm\n", result.TargetURL, result.BodyChars)
	}
	if result.NextCommand != "" {
		fmt.Println("next:", result.NextCommand)
	}
	return err
}

func needsShellQuote(r rune) bool {
	return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("-_./=:,@%+", r))
}

// confirmCommand repeats the previewed request with --confirm, so the posted
// text is the previewed text.
func confirmCommand(subcommand string, args []string) string {
	words := []string{"issueops", "remote", subcommand}
	for _, arg := range args {
		// A leading = is zsh's EQUALS expansion (=cmd becomes its path).
		if arg == "" || strings.HasPrefix(arg, "=") || strings.ContainsFunc(arg, needsShellQuote) {
			arg = shelltoken.QuoteWord(arg)
		}
		words = append(words, arg)
	}
	return strings.Join(append(words, "--confirm"), " ")
}
