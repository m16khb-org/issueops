package contractcli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	toolconformancecontract "issueops/internal/contract/toolconformance"
	toolconformancedomain "issueops/internal/domain/toolconformance"
	"os"
	"path/filepath"
	"sort"
	"strings"

	mcpcontract "issueops/internal/contract/mcp"
)

type LiveRequest struct {
	Hosts              []string
	Models             []string
	Profile            string
	Only               string
	ResumeReport       string
	TargetCompleted    int
	MaxAttemptsPerCase int
	EvidenceDir        string
	Previous           *toolconformancecontract.BenchmarkReport
}

type ReplayOutcome struct {
	HandlerCalls      int
	StateBeforeSHA256 string
	StateAfterSHA256  string
	Classification    toolconformancecontract.Classification
	Diagnostics       []toolconformancecontract.Diagnostic
	FinalResult       map[string]any
}

type ConformanceDependencies struct {
	LoadManifest          func([]toolconformancecontract.ToolDescriptor) ([]toolconformancecontract.Fixture, []toolconformancecontract.BaselineCase, error)
	LoadRegressionFixture func(string) (toolconformancecontract.RegressionFixture, error)
	ReplayRegression      func(toolconformancecontract.RegressionFixture, []toolconformancecontract.ToolDescriptor, string) (toolconformancecontract.ReplayResult, error)
	ServeProbe            func(context.Context, io.Reader, io.Writer, mcpcontract.ConformanceProbeConfig) error
	Catalog               func() []mcpcontract.Tool
	Root                  func() string
	RunProcess            func(context.Context, LiveRequest) (toolconformancecontract.BenchmarkReport, error)
	EvaluateBaseline      func() (caseCount int, ok bool, err error)
	Replay                func(context.Context, string, string) (ReplayOutcome, error)
}

type Conformance struct{ deps ConformanceDependencies }

func NewConformance(deps ConformanceDependencies) *Conformance {
	c := &Conformance{deps: deps}
	if c.deps.Catalog == nil {
		c.deps.Catalog = func() []mcpcontract.Tool { return nil }
	}
	if c.deps.Root == nil {
		c.deps.Root = func() string {
			root, err := os.Getwd()
			if err != nil {
				return "."
			}
			return root
		}
	}
	if c.deps.RunProcess == nil {
		c.deps.RunProcess = func(context.Context, LiveRequest) (toolconformancecontract.BenchmarkReport, error) {
			return toolconformancecontract.BenchmarkReport{}, fmt.Errorf("live_runner_unavailable")
		}
	}
	if c.deps.LoadManifest == nil {
		c.deps.LoadManifest = func([]toolconformancecontract.ToolDescriptor) ([]toolconformancecontract.Fixture, []toolconformancecontract.BaselineCase, error) {
			return nil, nil, fmt.Errorf("conformance manifest loader is not configured")
		}
	}
	if c.deps.LoadRegressionFixture == nil {
		c.deps.LoadRegressionFixture = func(string) (toolconformancecontract.RegressionFixture, error) {
			return toolconformancecontract.RegressionFixture{}, fmt.Errorf("conformance fixture loader is not configured")
		}
	}
	if c.deps.ReplayRegression == nil {
		c.deps.ReplayRegression = func(toolconformancecontract.RegressionFixture, []toolconformancecontract.ToolDescriptor, string) (toolconformancecontract.ReplayResult, error) {
			return toolconformancecontract.ReplayResult{}, fmt.Errorf("conformance replay is not configured")
		}
	}
	if c.deps.ServeProbe == nil {
		c.deps.ServeProbe = func(context.Context, io.Reader, io.Writer, mcpcontract.ConformanceProbeConfig) error {
			return fmt.Errorf("conformance probe is not configured")
		}
	}
	if c.deps.EvaluateBaseline == nil {
		c.deps.EvaluateBaseline = c.evaluateSyntheticBaseline
	}
	if c.deps.Replay == nil {
		c.deps.Replay = c.replayRegressionFixture
	}
	return c
}

func (c *Conformance) replayRegressionFixture(_ context.Context, fixturePath, stateDir string) (ReplayOutcome, error) {
	fixture, err := c.deps.LoadRegressionFixture(fixturePath)
	if err != nil {
		return ReplayOutcome{}, err
	}
	replayed, err := c.deps.ReplayRegression(fixture, c.conformanceDescriptors(), stateDir)
	if err != nil {
		return ReplayOutcome{}, err
	}
	if !replayed.OK {
		return ReplayOutcome{}, fmt.Errorf("replay_expectation_failed")
	}
	return ReplayOutcome{HandlerCalls: replayed.HandlerCalls, StateBeforeSHA256: replayed.StateBeforeSHA256, StateAfterSHA256: replayed.StateAfterSHA256, Classification: replayed.Classification, Diagnostics: replayed.Diagnostics, FinalResult: replayed.FinalResult}, nil
}

func (c *Conformance) Run(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing conformance subcommand")
	}
	switch args[0] {
	case "baseline":
		return c.runConformanceBaseline(args[1:])
	case "live":
		return c.runConformanceLive(args[1:])
	case "replay":
		return c.runConformanceReplay(args[1:])
	case "serve":
		return c.runConformanceServe(args[1:])
	default:
		return fmt.Errorf("unknown conformance subcommand %q", args[0])
	}
}

func (c *Conformance) conformanceDescriptors() []toolconformancecontract.ToolDescriptor {
	out := []toolconformancecontract.ToolDescriptor{}
	for _, t := range c.deps.Catalog() {
		out = append(out, toolconformancecontract.ToolDescriptor{Name: t.Name, InputSchema: t.InputSchema})
	}
	return out
}

func (c *Conformance) evaluateSyntheticBaseline() (int, bool, error) {
	fixtures, cases, err := c.deps.LoadManifest(c.conformanceDescriptors())
	if err != nil {
		return 0, false, err
	}
	byID := map[string]toolconformancecontract.Fixture{}
	schema := map[string]map[string]any{}
	byTool := map[string]map[string]any{}
	for _, d := range c.conformanceDescriptors() {
		byTool[d.Name] = d.InputSchema
	}
	for _, f := range fixtures {
		byID[f.ID] = f
		schema[f.ID] = byTool[f.SourceTool]
	}
	ok := true
	for _, c := range cases {
		raw, marshalErr := json.Marshal(c.Arguments)
		got, classifyErr := toolconformancedomain.Classify(toolconformancecontract.CallObservation{RawArguments: raw, CallCount: 1}, schema[c.FixtureID], byID[c.FixtureID].ExpectedArguments)
		ok = ok && marshalErr == nil && classifyErr == nil && got.Classification == c.ExpectedClassification
	}
	return len(cases), ok, nil
}

func (c *Conformance) runConformanceBaseline(args []string) error {
	fs := flag.NewFlagSet("contract conformance baseline", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	report, err := c.evaluateBaselineReport()
	if err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("cases=%d ok=%t\n", report.CaseCount, report.OK)
	}
	if !report.OK {
		return fmt.Errorf("baseline_failed")
	}
	return nil
}

func (c *Conformance) evaluateBaselineReport() (toolconformancecontract.BenchmarkReport, error) {
	caseCount, ok, err := c.deps.EvaluateBaseline()
	if err != nil {
		return toolconformancecontract.BenchmarkReport{}, err
	}
	regressions, err := regressionFixtures(regressionDirectory(c.deps.Root()))
	if err != nil {
		return toolconformancecontract.BenchmarkReport{}, err
	}
	for _, fixture := range regressions {
		outcome, replayErr := c.replayFixture(fixture)
		caseCount++
		ok = ok && replayErr == nil && outcome.HandlerCalls == 0 && outcome.StateBeforeSHA256 != "" && outcome.StateBeforeSHA256 == outcome.StateAfterSHA256
	}
	decision := toolconformancecontract.GateBaselinePassed
	if !ok {
		decision = toolconformancecontract.GateInconclusive
	}
	return toolconformancecontract.BenchmarkReport{
		OK: ok, SchemaVersion: toolconformancecontract.ReportSchemaVersion, RunID: "deterministic-baseline",
		Profile: "deterministic", CaseCount: caseCount,
		Counts: toolconformancecontract.BenchmarkCounts{Attempts: caseCount, Completed: caseCount},
		Gate:   toolconformancecontract.GateReport{Decision: decision},
		Hosts:  []toolconformancecontract.HostReport{}, Warnings: []string{},
	}, nil
}

func regressionDirectory(root string) string {
	return filepath.Join(root, "internal", "adapter", "toolconformance", "testdata", "regressions")
}

func regressionFixtures(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	fixtures := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			fixtures = append(fixtures, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Strings(fixtures)
	return fixtures, nil
}

func (c *Conformance) runConformanceLive(args []string) error {
	fs := flag.NewFlagSet("contract conformance live", flag.ContinueOnError)
	hosts := fs.String("hosts", "codex,claude", "comma-separated hosts")
	profile := fs.String("profile", "clean", "clean or context-pressure")
	only := fs.String("only", "", "host:fixture")
	resume := fs.String("resume-report", "", "previous report")
	target := fs.Int("target-completed", 1, "1, 10, or 20")
	maxAttempts := fs.Int("max-attempts-per-case", 3, "1 through 3")
	evidenceDir := fs.String("evidence-dir", ".issueops/evidence/tool-conformance", "ignored evidence directory")
	jsonOut := fs.Bool("json", false, "print JSON")
	var models stringSlice
	fs.Var(&models, "model", "host=value")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *profile != "clean" && *profile != "context-pressure" {
		return fmt.Errorf("invalid profile %q", *profile)
	}
	if *target != 1 && *target != 10 && *target != 20 {
		return fmt.Errorf("invalid target-completed %d", *target)
	}
	if *maxAttempts < 1 || *maxAttempts > 3 {
		return fmt.Errorf("invalid max-attempts-per-case %d", *maxAttempts)
	}
	request := LiveRequest{
		Hosts: splitNonEmpty(*hosts), Models: models, Profile: *profile,
		Only: *only, ResumeReport: *resume, TargetCompleted: *target,
		MaxAttemptsPerCase: *maxAttempts, EvidenceDir: *evidenceDir,
	}
	if len(request.Hosts) == 0 {
		return fmt.Errorf("hosts_required")
	}
	if os.Getenv("ISSUEOPS_TOOL_CONFORMANCE_LIVE") != "1" {
		return fmt.Errorf("live_opt_in_required")
	}
	baseline, err := c.evaluateBaselineReport()
	if err != nil || !baseline.OK {
		return fmt.Errorf("baseline_failed_before_live")
	}
	if request.ResumeReport != "" {
		previous, loadErr := loadBenchmarkReport(request.ResumeReport)
		if loadErr != nil {
			return loadErr
		}
		request.Previous = &previous
	}
	report, err := c.deps.RunProcess(context.Background(), request)
	if err != nil {
		return err
	}
	if err := c.persistLiveReport(&report, request); err != nil {
		return err
	}
	if *jsonOut {
		if err := printJSON(report); err != nil {
			return err
		}
	} else {
		fmt.Printf("gate=%s completed=%d attempts=%d report=%s\n", report.Gate.Decision, report.Counts.Completed, report.Counts.Attempts, report.Evidence.ReportPath)
	}
	if !report.OK {
		return fmt.Errorf("conformance_inconclusive")
	}
	return nil
}

func loadBenchmarkReport(path string) (toolconformancecontract.BenchmarkReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return toolconformancecontract.BenchmarkReport{}, err
	}
	var report toolconformancecontract.BenchmarkReport
	if err := json.Unmarshal(data, &report); err != nil {
		return toolconformancecontract.BenchmarkReport{}, err
	}
	if report.SchemaVersion != toolconformancecontract.ReportSchemaVersion {
		return toolconformancecontract.BenchmarkReport{}, fmt.Errorf("unsupported report schema version %d", report.SchemaVersion)
	}
	return report, nil
}

func (c *Conformance) persistLiveReport(report *toolconformancecontract.BenchmarkReport, request LiveRequest) error {
	root := c.deps.Root()
	evidenceRoot := request.EvidenceDir
	if !filepath.IsAbs(evidenceRoot) {
		evidenceRoot = filepath.Join(root, evidenceRoot)
	}
	relative, err := filepath.Rel(root, evidenceRoot)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence_dir_outside_issueops_root")
	}
	runDir := filepath.Join(evidenceRoot, safeRunID(report.RunID))
	if err := os.MkdirAll(runDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(runDir, 0o700); err != nil {
		return err
	}
	relativeRunDir := filepath.Join(relative, safeRunID(report.RunID))
	report.Evidence.ReportPath = filepath.Join(relativeRunDir, "report.json")
	if report.Gate.Decision == toolconformancecontract.GateAuthorizeHardening {
		candidate, tracked, buildErr := c.buildCandidateRegression(*report)
		if buildErr != nil {
			return buildErr
		}
		candidatePath := filepath.Join(runDir, "candidate-regression.json")
		if err := writePrivateJSONFile(candidatePath, candidate); err != nil {
			return err
		}
		report.Evidence.CandidateRegressionPath = filepath.Join(relativeRunDir, "candidate-regression.json")
		report.Evidence.TrackedFixturePath = tracked
	}
	return writePrivateJSONFile(filepath.Join(runDir, "report.json"), report)
}

func safeRunID(value string) string {
	var out strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			out.WriteRune(r)
		}
	}
	if out.Len() == 0 {
		return "run"
	}
	return out.String()
}

func writePrivateJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	parent := filepath.Dir(path)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(parent, ".report-")
	if err != nil {
		return err
	}
	name := temp.Name()
	defer os.Remove(name)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(append(data, '\n')); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func (c *Conformance) buildCandidateRegression(report toolconformancecontract.BenchmarkReport) (toolconformancecontract.RegressionFixture, string, error) {
	signature := report.Gate.ConfirmedSignature
	groups := map[string][]toolconformancecontract.EpisodeReport{}
	keys := []string{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			if episode.Status != "completed" || episode.DiagnosticSignature != signature {
				continue
			}
			key := host.Host + "\x00" + episode.FixtureID
			if _, exists := groups[key]; !exists {
				keys = append(keys, key)
			}
			groups[key] = append(groups[key], episode)
		}
	}
	sort.Strings(keys)
	matches := []toolconformancecontract.EpisodeReport{}
	for _, key := range keys {
		if len(groups[key]) >= 2 {
			matches = groups[key]
			break
		}
	}
	if len(matches) < 2 {
		return toolconformancecontract.RegressionFixture{}, "", fmt.Errorf("confirmed_signature_evidence_missing")
	}
	fixtures, _, err := c.deps.LoadManifest(c.conformanceDescriptors())
	if err != nil {
		return toolconformancecontract.RegressionFixture{}, "", err
	}
	var fixture toolconformancecontract.Fixture
	for _, candidate := range fixtures {
		if candidate.ID == matches[0].FixtureID {
			fixture = candidate
			break
		}
	}
	if fixture.ID == "" {
		return toolconformancecontract.RegressionFixture{}, "", fmt.Errorf("confirmed_fixture_missing")
	}
	evidenceIDs := make([]string, 0, len(matches))
	for _, match := range matches {
		if !validSHA256Text(match.EvidenceID) {
			return toolconformancecontract.RegressionFixture{}, "", fmt.Errorf("confirmed_evidence_id_invalid")
		}
		evidenceIDs = append(evidenceIDs, match.EvidenceID)
	}
	sort.Strings(evidenceIDs)
	modelLabel := matches[0].ObservedModel
	if modelLabel == "" {
		modelLabel = matches[0].RequestedModel
	}
	regression := toolconformancecontract.RegressionFixture{
		SchemaVersion: 1, FixtureID: fixture.ID, SourceTool: fixture.SourceTool, ProbeTool: fixture.ProbeTool,
		SourceSchemaSHA256: fixture.SchemaSHA256, Host: matches[0].Host,
		HostVersion: matches[0].HostVersion, ModelLabel: modelLabel,
		CanonicalArguments: matches[0].CanonicalArguments, RawArgumentsSHA256: matches[0].RawArgumentsSHA256,
		ExpectedClassification: matches[0].Classification, ExpectedDiagnostics: matches[0].Diagnostics,
		ExpectedDiagnosticSignature: signature, ConfirmedEvidenceIDs: evidenceIDs,
		ExpectedHandlerCallCount: 0,
		ExpectedFinalResult:      toolconformancedomain.InvalidToolArgumentsResult(fixture.SourceTool, matches[0].Diagnostics),
		ExpectedStateUnchanged:   true,
	}
	name := matches[0].Host + "-" + fixture.ID + "-" + firstN(signature, 12) + ".json"
	tracked := filepath.Join("internal", "adapter", "toolconformance", "testdata", "regressions", name)
	return regression, tracked, nil
}

func validSHA256Text(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
func firstN(value string, count int) string {
	if len(value) <= count {
		return value
	}
	return value[:count]
}

func (c *Conformance) runConformanceReplay(args []string) error {
	fs := flag.NewFlagSet("contract conformance replay", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	fixture := fs.String("fixture", "", "fixture")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *fixture == "" {
		return fmt.Errorf("fixture_required")
	}
	outcome, err := c.replayFixture(*fixture)
	if err != nil {
		return err
	}
	ok := outcome.HandlerCalls == 0 && outcome.StateBeforeSHA256 != "" && outcome.StateBeforeSHA256 == outcome.StateAfterSHA256
	result := map[string]any{
		"ok": ok, "classification": outcome.Classification, "diagnostics": outcome.Diagnostics,
		"handler_calls": outcome.HandlerCalls, "final_result": outcome.FinalResult,
		"state_digest": outcome.StateAfterSHA256,
	}
	if *jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	}
	if !ok {
		return fmt.Errorf("replay_failed")
	}
	return nil
}

func (c *Conformance) replayFixture(fixture string) (ReplayOutcome, error) {
	stateDir, err := os.MkdirTemp("", "issueops-conformance-replay-")
	if err != nil {
		return ReplayOutcome{}, err
	}
	defer os.RemoveAll(stateDir)
	if err := os.Chmod(stateDir, 0o700); err != nil {
		return ReplayOutcome{}, err
	}
	return c.deps.Replay(context.Background(), fixture, stateDir)
}

func (c *Conformance) runConformanceServe(args []string) error {
	fs := flag.NewFlagSet("contract conformance serve", flag.ContinueOnError)
	id := fs.String("fixture-id", "", "fixture id")
	path := fs.String("result-file", "", "result file")
	token := fs.String("run-token", "", "run token")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *id == "" || *path == "" || *token == "" {
		return fmt.Errorf("fixture-id, result-file, and run-token are required")
	}
	fixtures, _, err := c.deps.LoadManifest(c.conformanceDescriptors())
	if err != nil {
		return err
	}
	for _, f := range fixtures {
		if f.ID == *id {
			return c.deps.ServeProbe(context.Background(), os.Stdin, os.Stdout, mcpcontract.ConformanceProbeConfig{FixtureID: f.ID, ProbeTool: f.ProbeTool, Schema: c.sourceSchema(f.SourceTool), SchemaSHA: f.SchemaSHA256, ExpectedArguments: f.ExpectedArguments, ResultPath: *path, RunToken: *token})
		}
	}
	return fmt.Errorf("unknown fixture %s", *id)
}

func (c *Conformance) sourceSchema(name string) map[string]any {
	for _, t := range c.deps.Catalog() {
		if t.Name == name {
			copy, err := cloneJSONMap(t.InputSchema)
			if err != nil {
				return nil
			}
			return copy
		}
	}
	return nil
}

func cloneJSONMap(value map[string]any) (map[string]any, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var copy map[string]any
	if err := json.Unmarshal(b, &copy); err != nil {
		return nil, err
	}
	return copy, nil
}

type stringSlice []string

func (values *stringSlice) String() string { return strings.Join(*values, ",") }
func (values *stringSlice) Set(value string) error {
	if value == "" {
		return fmt.Errorf("empty value")
	}
	*values = append(*values, value)
	return nil
}

func splitNonEmpty(value string) []string {
	parts := strings.Split(value, ",")
	out := []string{}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
