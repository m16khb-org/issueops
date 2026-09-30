package hookcatalog

import (
	"flag"
	"io"
	"os"
	"strings"

	"issueops/cmd/issueops/hookcli/hookinput"
	hookcontract "issueops/internal/contract/hookprompt"
)

type Config struct {
	BuildCatalog  func(string) hookcontract.ProjectDocCatalogContext
	FormatContext func(host, eventName, additionalContext, userView string) map[string]any
	ResolveTarget func(string) string
	PrintJSON     func(any) error
}

// RunSessionStart renders the static project-doc catalog for every SessionStart
// source, including "compact". Claude Code 2.1.247 and Codex 0.150.1 both re-run
// SessionStart with source "compact" after compaction, and on both hosts only
// SessionStart output can carry model-facing additionalContext (verified against
// the installed binaries on 2026-08-27), so the catalog is re-established here
// rather than on PostCompact.
func RunSessionStart(args []string, config Config) error {
	fs := flag.NewFlagSet("hook session-start", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	hostFlag := fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	if err := recordLiveProbeSessionStart(stdin); err != nil {
		return err
	}
	cat := config.BuildCatalog(resolveRepo(*repo, stdin, config))
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	if !cat.ShouldInject {
		return config.PrintJSON(map[string]any{})
	}
	return config.PrintJSON(config.FormatContext(hostOf(hostFlag), "SessionStart", cat.Compact, cat.UserView))
}

// RunPostCompact keeps an explicit post-compaction catalog surface for hosts whose
// compaction event has no SessionStart re-run (Omo session_compact reads --json)
// and for diagnosis. Claude and Codex default installs do not register it: both
// hosts accept only user-facing output on PostCompact (Claude renders the raw
// stdout as a display message, Codex's post-compact.command.output schema has no
// hookSpecificOutput), so the host shape carries the readable catalog through
// systemMessage only.
func RunPostCompact(args []string, config Config) error {
	fs := flag.NewFlagSet("hook post-compact", flag.ContinueOnError)
	repo := fs.String("repo", "", "target repository path; defaults to hook stdin JSON or cwd")
	fs.String("host", "", "hook host (codex or claude); controls user-visible compatibility fields")
	jsonOut := fs.Bool("json", false, "print raw analysis JSON instead of host hook JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	stdin, _ := io.ReadAll(os.Stdin)
	cat := config.BuildCatalog(resolveRepo(*repo, stdin, config))
	if *jsonOut {
		return config.PrintJSON(cat)
	}
	if !cat.ShouldInject {
		return config.PrintJSON(map[string]any{})
	}
	return config.PrintJSON(map[string]any{"systemMessage": cat.UserView})
}

func resolveRepo(flagValue string, stdin []byte, config Config) string {
	repo := strings.TrimSpace(flagValue)
	if repo == "" {
		repo = hookinput.RepoFromHookInput(stdin)
	}
	if repo == "" {
		repo = config.ResolveTarget("")
	}
	return repo
}

func hostOf(hostFlag *string) string {
	return strings.ToLower(strings.TrimSpace(*hostFlag))
}
