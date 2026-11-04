package issueopsnext

// 모든 사례에 거는 불변식: 결과 모양, next_command 문법, lease 뷰, 경고.

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	clicontract "issueops/internal/contract/cli"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// ---------------------------------------------------------------- 명령 카탈로그

// usageEntry is one line of the CLI usage catalog (internal/contract/cli), the
// single source of the issueops command grammar.
type usageEntry struct {
	path     []string
	flags    map[string]bool
	required []string
	idFlag   string
	// actorFlags: the line lists ACTOR_FLAGS or RECORD_ACTOR_FLAGS.
	actorFlags bool
}

var (
	pathWord    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	flagPattern = regexp.MustCompile(`--[a-z][a-z0-9-]*`)
)

func parseUsageCatalog() []usageEntry {
	var entries []usageEntry
	for line := range strings.SplitSeq(clicontract.IssueOpsUsageCatalog, "\n") {
		if entry, ok := parseUsageLine(line); ok {
			entries = append(entries, entry)
		}
	}
	return entries
}

func parseUsageLine(line string) (usageEntry, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[0] != "issueops" {
		return usageEntry{}, false
	}
	entry := usageEntry{flags: map[string]bool{}, actorFlags: strings.Contains(line, "ACTOR_FLAGS")}
	rest := fields[1:]
	for len(rest) > 0 && pathWord.MatchString(rest[0]) {
		entry.path = append(entry.path, rest[0])
		rest = rest[1:]
	}
	entry.scanFlags(rest)
	return entry, true
}

// scanFlags records every flag of the usage tail and the ones outside any
// [optional] or (alternative|group) as required.
func (entry *usageEntry) scanFlags(tokens []string) {
	depth := 0
	for position, token := range tokens {
		depth += strings.Count(token, "[") + strings.Count(token, "(")
		for _, flag := range flagPattern.FindAllString(token, -1) {
			entry.flags[flag] = true
		}
		if depth == 0 && strings.HasPrefix(token, "--") && !strings.Contains(token, "|") {
			entry.required = append(entry.required, token)
			if (token == "--id" || token == "--parent") && position+1 < len(tokens) && tokens[position+1] == "ID" {
				entry.idFlag = token
			}
		}
		depth -= strings.Count(token, "]") + strings.Count(token, ")")
	}
}

func actorLegendFlags() map[string]bool {
	out := map[string]bool{}
	for _, flag := range flagPattern.FindAllString(clicontract.IssueOpsActorFlagLegend, -1) {
		out[flag] = true
	}
	return out
}

// subcommandFor picks the longest catalog path that prefixes the command words.
func subcommandFor(entries []usageEntry, words []string) (usageEntry, bool) {
	var best usageEntry
	found := false
	for _, entry := range entries {
		if len(entry.path) <= len(best.path) && found {
			continue
		}
		if len(entry.path) <= len(words) && slices.Equal(entry.path, words[:len(entry.path)]) {
			best, found = entry, true
		}
	}
	return best, found
}

// documentedPlaceholders are the `<...>` slots next_command may leave for the
// caller to fill; next_command_kind=template announces them.
var documentedPlaceholders = map[string]bool{
	"<TEXT>": true, "<URL>": true, "<PATH>": true, "<SHA>": true, "<LABEL>": true, "<USER>": true,
	"<BRANCH>": true, "<N>": true, "<source_root>": true, "<branch>": true,
	"<github|gitlab>": true, "<subagent|inline>": true, "<NAME>": true, "<REF>": true, "<worktree>": true,
	"<pass|revise|stop>": true, "<pr|mr>": true, "<bug|feature|proposal|implementation_task>": true,
	"<REVIEWER_EFFORT>": true, "<REVIEWER_HOST>": true, "<REVIEWER_MODEL>": true,
}

var placeholderPattern = regexp.MustCompile(`<[^<>\s]+>`)

// ---------------------------------------------------------------- 위반 수집

type matrixViolations struct {
	counts   map[string]int
	examples map[string][]string
}

func newMatrixViolations() *matrixViolations {
	return &matrixViolations{counts: map[string]int{}, examples: map[string][]string{}}
}

func (v *matrixViolations) add(rule string, m *matrix, c *matrixCase, detail string) {
	v.counts[rule]++
	if len(v.examples[rule]) < 3 {
		v.examples[rule] = append(v.examples[rule], detail+"\n    case: "+m.label(c.sel))
	}
}

func (v *matrixViolations) report(t *testing.T) {
	t.Helper()
	rules := make([]string, 0, len(v.counts))
	for rule := range v.counts {
		rules = append(rules, rule)
	}
	sort.Strings(rules)
	for _, rule := range rules {
		t.Errorf("invariant %q violated in %d cases; first:\n  %s", rule, v.counts[rule], strings.Join(v.examples[rule], "\n  "))
	}
}

// ---------------------------------------------------------------- 검사

type matrixChecker struct {
	m          *matrix
	usage      []usageEntry
	actorFlags map[string]bool
	violations *matrixViolations
	stages     map[string]int
	cases      int
}

func (k *matrixChecker) run(sel []int) {
	c := &matrixCase{sel: sel, input: k.m.build(sel)}
	k.cases++
	// Input immutability is checked on every 4th case: it marshals the record twice.
	heavy := k.cases%4 == 0
	var before []byte
	if heavy {
		before, _ = json.Marshal(c.input.Record)
	}
	if !k.classify(c) {
		return
	}
	k.stages[c.got.Stage.Key]++
	if heavy {
		if after, _ := json.Marshal(c.input.Record); string(before) != string(after) {
			k.violations.add("classify does not mutate its input record", k.m, c, "record changed during Classify")
		}
	}
	k.checkShape(c)
	k.checkCommands(c)
	k.checkLease(c)
	k.checkRouting(c)
	k.checkWarnings(c)
}

func (k *matrixChecker) classify(c *matrixCase) (ok bool) {
	defer func() {
		if recovered := recover(); recovered != nil {
			k.violations.add("classify does not panic", k.m, c, fmt.Sprint("panic: ", recovered))
			ok = false
		}
	}()
	c.got = Classify(c.input)
	if again := Classify(c.input); !reflect.DeepEqual(c.got, again) {
		k.violations.add("classify is deterministic", k.m, c, fmt.Sprintf("%+v != %+v", c.got, again))
	}
	return true
}

// producibleStages are the stage keys Classify itself can return; ambiguous and
// invalid are assigned by the selector before classification.
var producibleStages = []string{
	issueopsnextcontract.StageNone, issueopsnextcontract.StageBlockedPending, issueopsnextcontract.StageBlockedRoot,
	issueopsnextcontract.StageBlockedHolderLive, issueopsnextcontract.StageTakeover, issueopsnextcontract.StageClaim,
	issueopsnextcontract.StageIssue, issueopsnextcontract.StagePrepare, issueopsnextcontract.StagePlanWrite,
	issueopsnextcontract.StagePlanArchitecture, issueopsnextcontract.StagePlanReview, issueopsnextcontract.StagePlanHandoff,
	issueopsnextcontract.StageImplementEnter, issueopsnextcontract.StageImplement, issueopsnextcontract.StageClean,
	issueopsnextcontract.StageDocs, issueopsnextcontract.StageVerify, issueopsnextcontract.StageCommitPush,
	issueopsnextcontract.StagePRCreate, issueopsnextcontract.StagePRComplete, issueopsnextcontract.StageDone,
	issueopsnextcontract.StageCleanupResume, issueopsnextcontract.StageUnknown,
}

// fixedStageIndex are stages whose index is the user's ten-stage number.
var fixedStageIndex = map[string]int{
	issueopsnextcontract.StageNone: 0, issueopsnextcontract.StageIssue: 1, issueopsnextcontract.StagePrepare: 2,
	issueopsnextcontract.StagePlanWrite: 3, issueopsnextcontract.StagePlanArchitecture: 3,
	issueopsnextcontract.StagePlanReview: 3, issueopsnextcontract.StagePlanHandoff: 3,
	issueopsnextcontract.StageImplementEnter: 4, issueopsnextcontract.StageImplement: 4,
	issueopsnextcontract.StageClean: 5, issueopsnextcontract.StageDocs: 6, issueopsnextcontract.StageVerify: 7,
	issueopsnextcontract.StageCommitPush: 8, issueopsnextcontract.StagePRCreate: 9,
	issueopsnextcontract.StagePRComplete: 9, issueopsnextcontract.StageDone: 10,
}

func (k *matrixChecker) checkShape(c *matrixCase) {
	got := c.got
	if !slices.Contains(producibleStages, got.Stage.Key) {
		k.violations.add("result has a known stage key", k.m, c, fmt.Sprintf("stage %q", got.Stage.Key))
	}
	if got.Stage.Index < 0 || got.Stage.Index > 10 {
		k.violations.add("stage index is within 0..10", k.m, c, fmt.Sprintf("%s index %d", got.Stage.Key, got.Stage.Index))
	}
	if want, fixed := fixedStageIndex[got.Stage.Key]; fixed && got.Stage.Index != want {
		// merge-in-progress reuses implement/done with their own index; both match the table.
		k.violations.add("stage key and index agree", k.m, c, fmt.Sprintf("%s index %d want %d", got.Stage.Key, got.Stage.Index, want))
	}
	if strings.TrimSpace(got.NextCommand) == "" {
		k.violations.add("every result has a next_command", k.m, c, "stage "+got.Stage.Key+" has no next_command")
	}
	if strings.TrimSpace(c.input.Record.ID) != "" && got.Exits.AbandonCommand == "" {
		k.violations.add("a cycle always offers the abandon exit", k.m, c, "empty abandon_command")
	}
	k.checkCommandKind(c)
}

func (k *matrixChecker) checkCommandKind(c *matrixCase) {
	got := c.got
	template := strings.Contains(got.NextCommand, "<") || strings.Contains(got.NextCommand, "ACTOR_FLAGS")
	switch {
	case got.NextCommandKind != issueopsnextcontract.NextCommandKindTemplate && got.NextCommandKind != issueopsnextcontract.NextCommandKindExact:
		k.violations.add("next_command_kind is exact or template", k.m, c, fmt.Sprintf("kind %q for %q", got.NextCommandKind, got.NextCommand))
	case template != (got.NextCommandKind == issueopsnextcontract.NextCommandKindTemplate):
		k.violations.add("next_command_kind matches the remaining placeholders", k.m, c, fmt.Sprintf("kind %q for %q", got.NextCommandKind, got.NextCommand))
	}
}

func (k *matrixChecker) checkCommands(c *matrixCase) {
	k.checkCommand(c, "next_command", c.got.NextCommand)
	k.checkCommand(c, "exits.pause_command", c.got.Exits.PauseCommand)
	k.checkCommand(c, "exits.abandon_command", c.got.Exits.AbandonCommand)
	k.checkCommand(c, "exits.takeover_command", c.got.Exits.TakeoverCommand)
	if c.got.Exits.AbandonCommand != "" && !strings.Contains(c.got.Exits.AbandonCommand, " --preview") {
		k.violations.add("abandon exit is a preview", k.m, c, c.got.Exits.AbandonCommand)
	}
}

func (k *matrixChecker) checkCommand(c *matrixCase, field, command string) {
	if command == "" {
		return
	}
	rule := func(name string) string { return field + ": " + name }
	words := strings.Fields(command)
	if words[0] != "issueops" || len(words) < 2 {
		k.violations.add(rule("starts with the issueops binary"), k.m, c, command)
		return
	}
	k.checkCommandText(c, rule, command)
	k.checkGrammar(c, rule, command, words[1:])
}

var shellVariablePattern = regexp.MustCompile(`\$[A-Za-z_]+`)

// checkCommandText covers what a caller sees before parsing the command:
// whitespace, --json, leftover placeholders and shell variables.
func (k *matrixChecker) checkCommandText(c *matrixCase, rule func(string) string, command string) {
	if strings.Contains(command, "  ") || strings.TrimSpace(command) != command || strings.ContainsAny(command, "\n\t") {
		k.violations.add(rule("has no stray whitespace"), k.m, c, fmt.Sprintf("%q", command))
	}
	if !strings.Contains(command, " --json") && !strings.HasPrefix(command, "issueops start ") {
		k.violations.add(rule("asks for --json"), k.m, c, command)
	}
	k.checkPlaceholders(c, rule, command)
	for _, variable := range shellVariablePattern.FindAllString(command, -1) {
		if variable != "$ACTOR_FLAGS" && variable != "$RECORD_ACTOR_FLAGS" {
			k.violations.add(rule("uses only the documented shell variables"), k.m, c, variable+" in "+command)
		}
	}
}

func (k *matrixChecker) checkPlaceholders(c *matrixCase, rule func(string) string, command string) {
	generationless := c.input.Record.Execution != nil && c.input.Record.Execution.Lease.Generation == 0
	for _, placeholder := range placeholderPattern.FindAllString(command, -1) {
		if !documentedPlaceholders[placeholder] {
			k.violations.add(rule("leaves only documented placeholders"), k.m, c, placeholder+" in "+command)
		}
		if placeholder == "<N>" && !generationless {
			k.violations.add(rule("generation placeholder only without a generation"), k.m, c, command)
		}
	}
}

func (k *matrixChecker) checkGrammar(c *matrixCase, rule func(string) string, command string, words []string) {
	entry, ok := subcommandFor(k.usage, words)
	if !ok {
		// Root commands outside the issueops lifecycle catalog (gates, ...) are
		// checked by name only.
		if !slices.ContainsFunc(clicontract.RootCommands(), func(root clicontract.Command) bool { return root.Name == words[0] }) {
			k.violations.add(rule("names a real issueops subcommand"), k.m, c, command)
		}
		return
	}
	k.checkFlags(c, rule, command, entry, words)
	for _, required := range entry.required {
		if !slices.Contains(words, required) && !strings.Contains(command, required+"=") {
			k.violations.add(rule("carries the subcommand's required flags"), k.m, c, required+" missing in "+command)
		}
	}
	if entry.idFlag != "" && !addressesRecord(words, entry.idFlag) {
		k.violations.add(rule("addresses this record"), k.m, c, entry.idFlag+" is not "+matrixID+" in "+command)
	}
}

func (k *matrixChecker) checkFlags(c *matrixCase, rule func(string) string, command string, entry usageEntry, words []string) {
	actorFlags := strings.Contains(command, "ACTOR_FLAGS") || entry.actorFlags
	for _, word := range words[len(entry.path):] {
		name, _, _ := strings.Cut(word, "=")
		if strings.HasPrefix(word, "--") && !entry.flags[name] && !(actorFlags && k.actorFlags[name]) {
			k.violations.add(rule("uses only flags of that subcommand"), k.m, c, name+" is not a flag of issueops "+strings.Join(entry.path, " ")+": "+command)
		}
	}
}

// addressesRecord reports whether flag is followed by this record's id (the
// replace chain single-quotes it).
func addressesRecord(words []string, flag string) bool {
	index := slices.Index(words, flag)
	return index >= 0 && index+1 < len(words) && strings.Trim(words[index+1], "'") == matrixID
}

// ---------------------------------------------------------------- 경고

func (k *matrixChecker) checkWarnings(c *matrixCase) {
	in, got := c.input, c.got
	if strings.TrimSpace(in.Record.ID) == "" {
		return
	}
	has := func(fragment string) bool {
		return slices.ContainsFunc(got.Warnings, func(w string) bool { return strings.Contains(w, fragment) })
	}
	k.checkForwardedWarnings(c, has)
	if in.Record.Execution == nil {
		if has("canonical worktree is") || has("in progress in the canonical worktree") {
			k.violations.add("no worktree warnings without an execution", k.m, c, strings.Join(got.Warnings, "; "))
		}
		return
	}
	for _, rule := range worktreeWarningSpecs(in) {
		if has(rule.fragment) != rule.want {
			k.violations.add(rule.name, k.m, c, fmt.Sprintf("want=%v warnings=%v", rule.want, got.Warnings))
		}
	}
}

// checkForwardedWarnings: readiness warnings and unsettled child creates
// reach next whatever stage the record routes to.
func (k *matrixChecker) checkForwardedWarnings(c *matrixCase, has func(string) bool) {
	in := c.input
	if in.Local != nil {
		for _, warning := range in.Local.Warnings {
			if !slices.Contains(c.got.Warnings, warning) {
				k.violations.add("readiness warnings always reach next", k.m, c, warning)
			}
		}
	}
	for _, op := range in.Record.ChildCreateOperations {
		if intentIsUnsettled(&op.IssueOpsIssueCreateIntent) && !has("reconcile-child --id "+matrixID+" --operation-id "+op.OperationID) {
			k.violations.add("an unsettled child create always warns", k.m, c, op.OperationID)
		}
	}
}

type warningSpec struct {
	name, fragment string
	want           bool
}

// worktreeWarningSpecs states when each worktree warning must (and must not)
// appear for a record that has an execution.
func worktreeWarningSpecs(in Input) []warningSpec {
	execution := in.Record.Execution
	completion := execution.Completion
	gone := strings.TrimSpace(execution.Workspace.Root) != "" && !in.WorktreePresent && in.Record.CleanupAttempt == nil
	wrongBranch := in.WorktreePresent && in.WorktreeBranch != "" && in.WorktreeBranch != execution.Workspace.Branch
	detached := in.WorktreePresent && in.WorktreeBranch == "" && in.WorktreeHead != "" && in.GitOperation != "rebase"
	headMoved := completion != nil && in.WorktreeHead != "" && completion.FinalHead != "" && completion.FinalHead != in.WorktreeHead
	gitOpen := slices.Contains([]string{"rebase", "cherry-pick", "revert"}, in.GitOperation)
	return []warningSpec{
		{"missing-worktree warning appears exactly when the worktree is gone", "canonical worktree is missing", gone},
		{"wrong-branch warning appears exactly on a branch mismatch", "canonical worktree is on branch", wrongBranch},
		{"detached warning appears exactly on a detached head", "detached HEAD", detached},
		{"completion-head warning appears exactly when the worktree head moved", "differs from the recorded completion head", headMoved},
		{"open git operations always warn (merge has its own routing)", "in progress in the canonical worktree", gitOpen},
	}
}
