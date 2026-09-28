package architecture

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

var updateDDDInventory = flag.Bool("update-ddd-inventory", false, "record the current production responsibility inventory")

type dddSource struct {
	Path    string   `json:"path"`
	Owner   string   `json:"owner"`
	Task    string   `json:"task"`
	Symbols []string `json:"symbols"`
}

type dddArtifactEntry struct {
	Path  string `json:"path"`
	Owner string `json:"owner"`
	Task  string `json:"task"`
}

type dddInventory struct {
	Sources   []dddSource        `json:"sources"`
	Artifacts []dddArtifactEntry `json:"artifacts"`
	Policies  []dddPolicy        `json:"policies"`
}

type dddPolicy struct {
	ID               string   `json:"id"`
	SourcePath       string   `json:"source_path"`
	SourceSymbol     string   `json:"source_symbol"`
	Responsibility   string   `json:"responsibility"`
	Target           string   `json:"target"`
	TargetSymbol     string   `json:"target_symbol,omitempty"`
	Task             string   `json:"task"`
	Entrypoints      []string `json:"entrypoints"`
	EvidenceTests    []string `json:"evidence_tests"`
	Status           string   `json:"status"`
	SourceRetainedAs string   `json:"source_retained_as,omitempty"`
}

type dddContractFunction struct {
	Path   string `json:"path"`
	Symbol string `json:"symbol"`
	Role   string `json:"role"`
}

func TestDDDResponsibilityInventoryMatchesSource(t *testing.T) {
	root := findRepoRoot(t)
	got := collectDDDInventory(t, root)
	path := filepath.Join(root, "internal", "architecture", "testdata", "ddd_responsibility_inventory.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read responsibility inventory: %v", err)
	}
	var want dddInventory
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	got.Policies = want.Policies
	if *updateDDDInventory {
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		want = got
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("production file or symbol has no recorded responsibility; review the change and update the inventory")
	}
	if len(want.Policies) == 0 {
		t.Fatal("business policy ownership ledger is empty")
	}
	for _, artifact := range want.Artifacts {
		if artifact.Owner == "" || artifact.Task == "" {
			t.Errorf("unassigned non-Go artifact: %s", artifact.Path)
		}
	}
	byPath := make(map[string]dddSource, len(want.Sources))
	for _, source := range want.Sources {
		byPath[source.Path] = source
	}
	ids := map[string]bool{}
	for _, policy := range want.Policies {
		if policy.ID == "" || ids[policy.ID] || policy.Responsibility == "" || policy.Target == "" || policy.Task == "" ||
			len(policy.Entrypoints) == 0 || len(policy.EvidenceTests) == 0 {
			t.Errorf("incomplete or duplicate policy entry: %+v", policy)
		}
		ids[policy.ID] = true
		if policy.SourceRetainedAs != "" && policy.Status != "migrated" {
			t.Errorf("%s retained source requires a migrated domain policy", policy.ID)
		}
		if policy.Status != "migrate" && policy.Status != "migrated" && policy.Status != "retain" {
			t.Errorf("%s has invalid status %q", policy.ID, policy.Status)
		}
		if policy.Status == "migrate" && !slicesContains(byPath[policy.SourcePath].Symbols, policy.SourceSymbol) {
			t.Errorf("%s source symbol %s is absent from %s", policy.ID, policy.SourceSymbol, policy.SourcePath)
		}
		if policy.Status == "migrated" {
			if policy.TargetSymbol == "" {
				t.Errorf("%s migrated policy has no target symbol", policy.ID)
			}
			sourceExists := slicesContains(byPath[policy.SourcePath].Symbols, policy.SourceSymbol)
			if policy.SourceRetainedAs != "" {
				// A use-case method can retain I/O sequencing after its decision moves
				// to domain. Its behavior tests must prove the domain decision is used.
				if policy.SourceRetainedAs != "application-orchestration" || !sourceExists || byPath[policy.SourcePath].Owner != "application" || !strings.HasPrefix(policy.Target, "internal/domain/") {
					t.Errorf("%s has invalid retained source responsibility", policy.ID)
				}
			} else if sourceExists {
				t.Errorf("%s migrated policy remains in %s", policy.ID, policy.SourcePath)
			}
			found := false
			for _, source := range want.Sources {
				if strings.HasPrefix(source.Path, policy.Target+"/") && slicesContains(source.Symbols, policy.TargetSymbol) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s migrated target symbol %s is absent from %s", policy.ID, policy.TargetSymbol, policy.Target)
			}
		}
	}
}

func TestDDDContractFunctionsHaveExplicitRoles(t *testing.T) {
	root := findRepoRoot(t)
	inventory := collectDDDInventory(t, root)
	data, err := os.ReadFile(filepath.Join(root, "internal", "architecture", "testdata", "ddd_contract_function_roles.json"))
	if err != nil {
		t.Fatal(err)
	}
	var roles []dddContractFunction
	if err := json.Unmarshal(data, &roles); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{}
	for _, source := range inventory.Sources {
		if !strings.HasPrefix(source.Path, "internal/contract/") {
			continue
		}
		for _, symbol := range source.Symbols {
			if strings.HasPrefix(symbol, "func ") {
				want[source.Path+"|"+symbol] = false
			}
		}
	}
	for _, role := range roles {
		key := role.Path + "|" + role.Symbol
		seen, exists := want[key]
		if !exists || seen {
			t.Errorf("unknown or duplicate contract function role: %s", key)
			continue
		}
		if role.Role != "shape" && role.Role != "codec" && role.Role != "clone" && role.Role != "error" && role.Role != "projection" && role.Role != "mixed-deferred" {
			t.Errorf("%s has unsupported contract role %q", key, role.Role)
		}
		if role.Role == "mixed-deferred" && role.Path != "internal/contract/issueops/record_validation.go" &&
			role.Path != "internal/contract/issueops/execution.go" &&
			role.Path != "internal/contract/issueops/capability_baseline.go" &&
			role.Path != "internal/contract/issueops/issue_create.go" {
			t.Errorf("new mixed contract policy needs an explicit T02 migration decision: %s", key)
		}
		want[key] = true
	}
	for key, seen := range want {
		if !seen {
			t.Errorf("contract function has no reviewed role: %s", key)
		}
	}
}

func slicesContains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func TestDDDInventoryFindsNewPolicySymbol(t *testing.T) {
	before := []byte("package policy\nfunc Existing() {}\n")
	after := []byte("package policy\nfunc Existing() {}\nfunc NewDecision() {}\n")
	want := []string{"func Existing"}
	if got := declarationsInGo(t, "policy.go", before); !reflect.DeepEqual(got, want) {
		t.Fatalf("before declarations = %v, want %v", got, want)
	}
	if got := declarationsInGo(t, "policy.go", after); reflect.DeepEqual(got, want) {
		t.Fatalf("new policy symbol was not detected: %v", got)
	}
}

func TestDDDOwnerRoutesKnownMigrationTargets(t *testing.T) {
	cases := map[string]string{
		"internal/contract/issueopsreview/gates.go":                      "T03",
		"internal/domain/issueopsreview/gates.go":                        "T03",
		"internal/application/issueopsreview/record.go":                  "T03",
		"internal/port/issueopsreview/store.go":                          "T03",
		"internal/domain/issueops/record_invariants.go":                  "T02",
		"internal/domain/looprun/lifecycle.go":                           "T14",
		"internal/domain/policy/command_decision.go":                     "T09",
		"internal/application/policy/service.go":                         "T09",
		"internal/application/audit/service.go":                          "T09",
		"internal/contract/policy/overrides.go":                          "T09",
		"internal/domain/issueops/phase_ledger.go":                       "T04",
		"internal/contract/issueopspreparation/planner_gates.go":         "T02",
		"internal/adapter/issueops/devilsadvocate/devils_advocate.go":    "T03",
		"internal/adapter/issueops/issueops_regress.go":                  "T03",
		"internal/adapter/issueops/issueops_phase.go":                    "T04",
		"internal/adapter/outbound/issueopslease/sqlite.go":              "T05",
		"internal/adapter/issueops/execution_sync_base.go":               "T06",
		"internal/adapter/issueops/execution_publication_store.go":       "T07",
		"internal/adapter/issueops/child_scan.go":                        "T08",
		"internal/adapter/policy/policy_evaluate.go":                     "T09",
		"internal/adapter/gates/check.go":                                "T10",
		"cmd/issueops/installcli/install.go":                             "T11",
		"cmd/issueops/updatecli/update_bootstrap.go":                     "T12",
		"internal/adapter/projectdocs/project_docs_route.go":             "T13",
		"internal/adapter/worker/worker.go":                              "T14",
		"internal/adapter/toolconformance/benchmark.go":                  "T15",
		"cmd/issueops/riskqa/risk_qa_plan.go":                            "T16",
		"cmd/issueops/selfworkflow/summary/self_verify_summary_score.go": "T17",
		"cmd/issueops/selfworkflow/steps/self_verify_steps.go":           "T18",
		"internal/domain/mcp/catalog.go":                                 "T19",
		"cmd/issueops/issueopsapp/issueops_next_wiring.go":               "T20",
	}
	for path, want := range cases {
		_, task := dddOwner(path)
		if task != want {
			t.Errorf("%s: task = %s, want %s", path, task, want)
		}
	}
}

func TestDDDExecutableArtifactsRequireExplicitOwnership(t *testing.T) {
	for path, wantTask := range map[string]string{
		"scripts/install-native.sh":     "T11",
		"scripts/measure_efficiency.py": "T17",
		"configs/upstream.json":         "T12",
		"configs/omo/issueops.js":       "T19",
		"skills/issueops/SKILL.md":      "T21",
	} {
		entry, ok := dddArtifactOwner(path)
		if !ok || entry.Task != wantTask || entry.Owner == "" {
			t.Errorf("%s = %+v, %t; want task %s", path, entry, ok, wantTask)
		}
	}
	if _, ok := dddArtifactOwner("scripts/new-unknown.sh"); ok {
		t.Fatal("unknown executable script received implicit ownership")
	}
}

func collectDDDInventory(t *testing.T, root string) dddInventory {
	t.Helper()
	result := dddInventory{Sources: []dddSource{}, Artifacts: []dddArtifactEntry{}}
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			owner, task := dddOwner(rel)
			result.Sources = append(result.Sources, dddSource{
				Path: rel, Owner: owner, Task: task, Symbols: declarationsInGo(t, path, data),
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, top := range []string{"scripts", "configs", "skills"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !dddArtifact(top, path) {
				return nil
			}
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			artifact, ok := dddArtifactOwner(filepath.ToSlash(rel))
			if !ok {
				t.Errorf("non-Go artifact has no owner or task: %s", rel)
				return nil
			}
			result.Artifacts = append(result.Artifacts, artifact)
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Slice(result.Sources, func(i, j int) bool { return result.Sources[i].Path < result.Sources[j].Path })
	sort.Slice(result.Artifacts, func(i, j int) bool { return result.Artifacts[i].Path < result.Artifacts[j].Path })
	return result
}

func declarationsInGo(t *testing.T, path string, data []byte) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
	if err != nil {
		t.Fatal(err)
	}
	declarations := []string{}
	for _, declaration := range file.Decls {
		switch decl := declaration.(type) {
		case *ast.FuncDecl:
			name := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				var out bytes.Buffer
				if err := formatReceiver(&out, decl.Recv.List[0].Type); err != nil {
					t.Fatal(err)
				}
				name = out.String() + "." + name
			}
			declarations = append(declarations, "func "+name)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					declarations = append(declarations, "type "+typ.Name.Name)
				}
			}
		}
	}
	sort.Strings(declarations)
	return declarations
}

func formatReceiver(out *bytes.Buffer, expr ast.Expr) error {
	switch v := expr.(type) {
	case *ast.StarExpr:
		return formatReceiver(out, v.X)
	case *ast.Ident:
		_, err := out.WriteString(v.Name)
		return err
	case *ast.IndexExpr:
		return formatReceiver(out, v.X)
	case *ast.IndexListExpr:
		return formatReceiver(out, v.X)
	default:
		return fmt.Errorf("unsupported receiver type %T", expr)
	}
}

func dddOwner(path string) (string, string) {
	task := dddTask(path)
	switch {
	case strings.HasPrefix(path, "internal/contract/"):
		return "contract", task
	case strings.HasPrefix(path, "internal/domain/"):
		return "domain", task
	case strings.HasPrefix(path, "internal/application/"):
		return "application", task
	case strings.HasPrefix(path, "internal/port/"):
		return "port", task
	case strings.HasPrefix(path, "internal/architecture/"):
		return "architecture-test", task
	case strings.HasPrefix(path, "internal/adapter/"):
		return "adapter", task
	case strings.HasPrefix(path, "cmd/issueops/issueopsapp/"):
		return "composition", task
	default:
		return "inbound", task
	}
}

func dddTask(path string) string {
	for _, routed := range []struct {
		prefix string
		task   string
	}{
		{"internal/application/issueopsreview/plan_binding.go", "T07"},
		{"internal/domain/issueopsreview/plan_binding.go", "T07"},
		{"internal/contract/issueopsreview/", "T03"},
		{"internal/domain/issueopsreview/", "T03"},
		{"internal/application/issueopsreview/", "T03"},
		{"internal/port/issueopsreview/", "T03"},
		{"internal/domain/issueopsintent/", "T03"},
		{"internal/domain/issueops/record_invariants", "T02"},
		{"internal/domain/issueops/issue_create_invariants", "T02"},
		{"internal/domain/issueopslease/record_validation.go", "T02"},
		{"internal/domain/issueopslease/resume", "T06"},
		{"internal/domain/issueopslease/reconcile", "T06"},
		{"internal/domain/issueopslease/reseed", "T06"},
		{"internal/domain/issueopspreparation/resume_", "T06"},
		{"internal/domain/issueopspreparation/reconcile_", "T06"},
		{"internal/application/issueopslease/resume", "T06"},
		{"internal/application/issueopslease/reconcile", "T06"},
		{"internal/application/issueopslease/reseed", "T06"},
		{"internal/adapter/outbound/issueopslease/resume_", "T06"},
		{"internal/adapter/outbound/issueopslease/reconcile_", "T06"},
		{"internal/adapter/outbound/issueopslease/reseed_", "T06"},
		{"cmd/issueops/issueopsapp/issueops_resume_", "T06"},
		{"cmd/issueops/issueopsapp/issueops_reconcile_", "T06"},
		{"internal/adapter/outbound/issueopsrecord/lease_codec.go", "T02"},
		{"internal/domain/issueops/execution_validation", "T02"},
		{"internal/domain/issueops/evidence_phase", "T03"},
		{"internal/domain/issueops/phase_", "T04"},
		{"internal/domain/issueops/readiness_", "T04"},
		{"internal/domain/issueops/issue_create_transition", "T02"},
		{"internal/domain/issueops/issue_create_lifecycle", "T07"},
		{"internal/domain/issueops/issue_create_request.go", "T07"},
		{"internal/domain/issueops/record_provider.go", "T07"},
		{"internal/domain/issueops/issue_graph_sync.go", "T07"},
		{"internal/adapter/issueops/issue_graph_poster.go", "T07"},
		{"internal/adapter/issueops/issue_provider_observation.go", "T07"},
		{"internal/domain/issueops/native_actor.go", "T05"},
		{"internal/application/issueopscycle/native_actor.go", "T05"},
		{"internal/domain/issueops/remote_command_defaults.go", "T07"},
		{"internal/domain/issueops/remote_completion.go", "T07"},
		{"internal/domain/issueops/review_reflection.go", "T07"},
		{"internal/adapter/issueops/review_plan_source.go", "T07"},
		{"internal/application/issueopscycle/mutation_authority.go", "T05"},
		{"internal/domain/issueopsauthorization/", "T05"},
		{"internal/contract/issueops/remote_completion.go", "T07"},
		{"internal/adapter/issueops/remote_record_store.go", "T07"},
		{"internal/adapter/issueops/completion_artifacts.go", "T07"},
		{"internal/domain/issueopsremote/create_metadata.go", "T07"},
		{"internal/domain/issueopsremote/issue_create_failure.go", "T07"},
		{"internal/domain/issueopsremote/publication_diagnostic.go", "T07"},
		{"internal/domain/issueopsremote/score_summary.go", "T07"},
		{"internal/domain/policy/remote_create_inputs.go", "T07"},
		{"internal/adapter/issueops/issue_creation_environment.go", "T07"},
		{"internal/domain/issueops/issue_url.go", "T08"},
		{"internal/domain/issueops/start.go", "T08"},
		{"internal/domain/issueops/branch.go", "T08"},
		{"internal/domain/issueops/branch_retarget.go", "T08"},
		{"internal/domain/issueops/workspace_link.go", "T08"},
		{"internal/domain/issueops/delegation.go", "T08"},
		{"internal/domain/issueops/child_status.go", "T08"},
		{"internal/domain/issueops/cleanup_status.go", "T08"},
		{"internal/domain/issueops/cleanup_finish.go", "T08"},
		{"internal/domain/issueops/linked_branch_audit.go", "T08"},
		{"internal/domain/issueops/linked_branch_cleanup.go", "T08"},
		{"internal/adapter/issueops/linked_branch_remote_ref.go", "T08"},
		{"internal/contract/issueops/cleanup_finish_inventory.go", "T08"},
		{"internal/adapter/issueops/cleanup_finish_environment.go", "T08"},
		{"internal/domain/issueops/cleanup_children.go", "T08"},
		{"internal/domain/issueopsremote/cleanup_child.go", "T08"},
		{"internal/application/issueopscleanup/", "T08"},
		{"internal/adapter/issueops/cleanup_status_environment.go", "T08"},
		{"internal/domain/issueops/child_verdict.go", "T08"},
		{"internal/domain/issueops/umbrella_topology.go", "T08"},
		{"internal/adapter/issueops/child_scan.go", "T08"},
		{"internal/domain/issueopsreview/inherited_review.go", "T08"},
		{"internal/application/issueopsdelegation/", "T08"},
		{"internal/adapter/issueops/child_cycle_store.go", "T08"},
		{"cmd/issueops/issueopsapp/issueops_child_start_wiring.go", "T08"},
		{"internal/domain/issueopsauthorization/plan_link.go", "T08"},
		{"internal/application/issueopscycle/plan_link_authority.go", "T08"},
		{"internal/adapter/issueops/link_environment.go", "T08"},
		{"internal/domain/issueops/issue_link.go", "T08"},
		{"internal/domain/issueopsremote/child_link.go", "T08"},
		{"cmd/issueops/issueopsapp/issueops_link_wiring.go", "T08"},
		{"internal/domain/issueops/branch_prepare.go", "T08"},
		{"internal/domain/issueopsremote/branch_prepare.go", "T08"},
		{"internal/adapter/issueops/branch_preparation_environment.go", "T08"},
		{"internal/adapter/issueops/branchinstructions/", "T08"},
		{"cmd/issueops/issueopsapp/issueops_branch_prepare_wiring.go", "T08"},
		{"cmd/issueops/issueopsapp/issueops_branch_retarget_wiring.go", "T08"},
		{"internal/application/issueopsbranch/", "T08"},
		{"internal/adapter/issueops/cycle_record_store.go", "T08"},
		{"internal/domain/issueops/publication_transition.go", "T07"},
		{"internal/domain/issueopspublication/", "T07"},
		{"internal/application/issueopspublication/", "T07"},
		{"internal/domain/issueopsbodysync/", "T07"},
		{"internal/domain/issueops/managed_body.go", "T07"},
		{"internal/domain/issueopsremote/artifact.go", "T07"},
		{"internal/domain/issueopsremote/create_preparation.go", "T07"},
		{"internal/domain/issueops/issue_reconcile.go", "T07"},
		{"internal/adapter/issueops/issue_create_candidates.go", "T07"},
		{"internal/application/issueopsremote/", "T07"},
		{"internal/application/issueopsbodysync/", "T07"},
		{"internal/application/issueopscompletion/", "T07"},
		{"internal/domain/state/prune.go", "T14"},
		{"internal/domain/looprun/", "T14"},
		{"internal/application/looprun/", "T14"},
		{"internal/domain/policy/", "T09"},
		{"internal/application/policy/", "T09"},
		{"internal/application/audit/", "T09"},
		{"internal/domain/preflight/", "T09"},
		{"internal/application/preflight/", "T09"},
		{"internal/domain/gates/", "T10"},
		{"internal/application/gates/", "T10"},
		{"internal/domain/guard/", "T10"},
		{"internal/application/guard/", "T10"},
		{"internal/domain/install/", "T11"},
		{"internal/domain/nativeactivation/", "T11"},
		{"internal/application/install/", "T11"},
		{"internal/domain/projectdoc/route.go", "T13"},
		{"internal/domain/toolconformance/gate.go", "T15"},
		{"internal/contract/riskqa/", "T16"},
		{"internal/contract/policy/overrides.go", "T09"},
		{"internal/domain/riskqa/", "T16"},
		{"internal/domain/selfverify/goal_score.go", "T17"},
	} {
		if strings.HasPrefix(path, routed.prefix) {
			return routed.task
		}
	}
	if strings.HasPrefix(path, "internal/contract/") {
		return "T02"
	}
	if strings.HasPrefix(path, "internal/architecture/") {
		return "T21"
	}
	if strings.HasPrefix(path, "internal/adapter/issueops/") {
		part := strings.TrimPrefix(path, "internal/adapter/issueops/")
		switch {
		case strings.HasPrefix(part, "benchmark/"):
			return "T17"
		case strings.HasPrefix(part, "devilsadvocate/"), strings.HasPrefix(part, "intentdesign/"), strings.HasPrefix(part, "compatibilityreview/"),
			strings.HasPrefix(part, "issueops_regress"), strings.HasPrefix(part, "issueops_implementation_review"),
			strings.HasPrefix(part, "issueops_project_docs_review"), strings.HasPrefix(part, "issueops_schema_evidence"),
			strings.HasPrefix(part, "issueops_feedback"), strings.HasPrefix(part, "issueops_ledger_recorders"):
			return "T03"
		case strings.HasPrefix(part, "issueops_phase"), strings.HasPrefix(part, "issueops_readiness"),
			strings.HasPrefix(part, "issueops_pr_readiness"), strings.HasPrefix(part, "gatesgate/"), strings.HasPrefix(part, "loopgate/"):
			return "T04"
		case strings.HasPrefix(part, "execution_lease"), strings.HasPrefix(part, "execution_prepare"):
			return "T05"
		case strings.HasPrefix(part, "execution_resume"), strings.HasPrefix(part, "execution_reconcile"),
			strings.HasPrefix(part, "execution_orca_intent"), strings.HasPrefix(part, "execution_sync_base"),
			strings.HasPrefix(part, "execution_mode_switch"):
			return "T06"
		case strings.HasPrefix(part, "execution_remote"), strings.HasPrefix(part, "execution_publication"), strings.HasPrefix(part, "issue_create_intent"),
			strings.HasPrefix(part, "issueops_remote"), strings.HasPrefix(part, "issueops_body_sync"), strings.HasPrefix(part, "issueops_completion_remote"),
			strings.HasPrefix(part, "issueops_devilsadvocate_reflect"):
			return "T07"
		case strings.HasPrefix(part, "start/"), strings.HasPrefix(part, "branchprepare/"),
			strings.HasPrefix(part, "linking/"), strings.HasPrefix(part, "delegation/"),
			strings.HasPrefix(part, "cleanup"), strings.HasPrefix(part, "orphancleanup/"),
			strings.HasPrefix(part, "issueops_delegation"), strings.HasPrefix(part, "issueops_child_gate"),
			strings.HasPrefix(part, "issueops_umbrella_topology"), strings.HasPrefix(part, "issueops_linked_branch"):
			return "T08"
		}
	}
	for _, item := range []struct {
		prefix string
		task   string
	}{
		{"internal/adapter/outbound/issueopslease/", "T05"},
		{"internal/adapter/outbound/issueopspreparation/", "T05"},
		{"internal/adapter/outbound/issueopspublication/", "T07"},
		{"internal/adapter/outbound/issueopscompletion/", "T07"},
		{"internal/adapter/policy/", "T09"}, {"internal/adapter/preflight/", "T09"},
		{"internal/adapter/audit/", "T09"}, {"internal/adapter/guard/", "T10"},
		{"internal/adapter/gates/", "T10"}, {"internal/adapter/install/", "T11"},
		{"internal/adapter/outbound/nativeactivation/", "T11"},
		{"internal/adapter/projectdocs/", "T13"}, {"internal/adapter/projectbootstrap/", "T13"},
		{"internal/adapter/looprun/", "T14"}, {"internal/adapter/worker/", "T14"},
		{"internal/adapter/lifecycle/", "T14"}, {"internal/adapter/channel/", "T14"},
		{"internal/adapter/operationalhealth/", "T15"}, {"internal/adapter/doctor/", "T15"},
		{"internal/adapter/trace/", "T15"}, {"internal/adapter/toolconformance/", "T15"},
		{"internal/adapter/hostprobe/", "T15"}, {"internal/adapter/commitsuggest/", "T15"},
		{"internal/adapter/lintdiagnose/", "T15"},
		{"cmd/issueops/installcli/", "T11"}, {"cmd/issueops/updatecli/", "T12"},
		{"cmd/issueops/apidoc/", "T16"}, {"cmd/issueops/qualitycli/", "T16"},
		{"cmd/issueops/riskqa/", "T16"},
		{"cmd/issueops/selfworkflow/summary/", "T17"},
		{"cmd/issueops/selfworkflow/augmentplan/", "T17"},
		{"cmd/issueops/selfworkflow/augmentcatalog/", "T17"},
		{"cmd/issueops/selfworkflow/historycompare/", "T17"},
		{"cmd/issueops/selfworkflow/candidateexport/", "T17"},
		{"cmd/issueops/selfworkflow/", "T18"},
		{"cmd/issueops/validationcli/", "T18"},
		{"internal/domain/mcp/", "T19"}, {"internal/domain/cli/", "T19"},
		{"internal/domain/nativehost/", "T19"}, {"internal/domain/omolifecycle/", "T19"},
		{"internal/domain/hook/", "T19"},
	} {
		if strings.HasPrefix(path, item.prefix) {
			return item.task
		}
	}
	return "T20"
}

func dddArtifact(top, path string) bool {
	switch top {
	case "scripts":
		return strings.HasSuffix(path, ".sh") || strings.HasSuffix(path, ".py")
	case "configs":
		return true
	case "skills":
		return filepath.Base(path) == "SKILL.md"
	default:
		return false
	}
}

func dddArtifactOwner(path string) (dddArtifactEntry, bool) {
	entry := dddArtifactEntry{Path: path}
	switch {
	case strings.HasPrefix(path, "skills/") && strings.HasSuffix(path, "/SKILL.md"):
		entry.Owner, entry.Task = "skill-contract", "T21"
	case strings.HasPrefix(path, "configs/"):
		entry.Owner, entry.Task = "host-configuration", "T19"
		if path == "configs/upstream.json" {
			entry.Task = "T12"
		}
	case strings.HasPrefix(path, "scripts/"):
		entry.Owner = "execution-script"
		switch path {
		case "scripts/install-native.sh":
			entry.Task = "T11"
		case "scripts/sync-glab-mcp.sh", "scripts/release-build-matrix.sh", "scripts/release-repro-smoke.sh":
			entry.Task = "T12"
		case "scripts/measure_efficiency.py", "scripts/measure_efficiency_test.py":
			entry.Task = "T17"
		case "scripts/verify-child-host-smoke.sh", "scripts/verify-go-test-match.sh", "scripts/verify-go-test-match-test.sh":
			entry.Task = "T20"
		case "scripts/meeting_notes_quality_rubric.py", "scripts/meeting_notes_skill_contract_test.py",
			"scripts/validate-skill.py", "scripts/validate_skill_test.py", "scripts/verify-skill-shell.py", "scripts/verify_skill_shell_test.py":
			entry.Task = "T21"
		}
	}
	return entry, entry.Owner != "" && entry.Task != ""
}
