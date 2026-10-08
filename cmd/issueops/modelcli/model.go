// Package modelcli owns the flags and output of `issueops model`. The
// composition root supplies file access and role-agent rendering through
// Dependencies.
package modelcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"strings"

	"issueops/cmd/issueops/jsonout"
	agentmodelapp "issueops/internal/application/agentmodel"
	contract "issueops/internal/contract/agentmodel"
	domain "issueops/internal/domain/agentmodel"
)

// UsageError is an input error (exit 2). Settings file and git errors exit 1.
type UsageError struct{ Message string }

func (e UsageError) Error() string { return e.Message }

// Dependencies are the operations `issueops model` needs.
type Dependencies struct {
	Models        agentmodelapp.Service
	WriteGlobal   func(path string, cfg contract.Config) error
	WriteLocal    func(ctx context.Context, repo string, cfg contract.Config) (excluded bool, err error)
	CodexCatalog  func() []string
	RoleAgentArgs func(ctx context.Context, host, repo string) ([]string, error)
	PrintArgv     func(host, model, effort string) []string
	Getwd         func() (string, error)
	Stdout        io.Writer
}

const usage = `Usage:
  issueops model show [--host claude|codex|omo|omp] [--repo PATH] [--json]
  issueops model set --scope global|local --host claude|codex --role ROLE [--model MODEL] [--effort EFFORT] [--repo PATH] [--json]
  issueops model unset --scope global|local --host claude|codex --role ROLE [--field model|effort] [--repo PATH] [--json]
  issueops model resolve --host claude|codex|omo|omp --role ROLE [--tier TIER] [--round N] [--model MODEL] [--effort EFFORT] [--agents] [--repo PATH] [--json]

Roles: implement, child-implement, plan-review, diff-review, review-escalate, research, reader-check.
Precedence per field: flag > local > global > built-in default.
Exit codes: 0 ok, 1 settings file or git error, 2 usage error.
`

func Run(deps Dependencies, args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Fprint(deps.Stdout, usage)
		return nil
	}
	ctx := context.Background()
	switch args[0] {
	case "show":
		return runShow(ctx, deps, args[1:])
	case "set":
		return runSet(ctx, deps, args[1:])
	case "unset":
		return runUnset(ctx, deps, args[1:])
	case "resolve":
		return runResolve(ctx, deps, args[1:])
	default:
		return UsageError{Message: fmt.Sprintf("unknown model subcommand %q\n%s", args[0], usage)}
	}
}

func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet("model "+name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func parse(fs *flag.FlagSet, args []string) error {
	if err := fs.Parse(args); err != nil {
		return UsageError{Message: err.Error()}
	}
	if fs.NArg() > 0 {
		return UsageError{Message: fmt.Sprintf("unexpected positional argument %q", fs.Arg(0))}
	}
	return nil
}

func repoOrCwd(deps Dependencies, repo string) (string, error) {
	if strings.TrimSpace(repo) != "" {
		return repo, nil
	}
	return deps.Getwd()
}

type showResult struct {
	OK         bool                             `json:"ok"`
	LocalPath  string                           `json:"local_path"`
	GlobalPath string                           `json:"global_path"`
	Hosts      map[string][]contract.Resolution `json:"hosts"`
}

func runShow(ctx context.Context, deps Dependencies, args []string) error {
	fs := newFlagSet("show")
	host := fs.String("host", "", "only this host")
	repoFlag := fs.String("repo", "", "repository (defaults to cwd)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	hosts := []string{"claude", "codex"}
	if *host != "" {
		if !domain.KnownHost(*host) {
			return UsageError{Message: fmt.Sprintf("unknown host %q (use claude, codex, omo, or omp)", *host)}
		}
		hosts = []string{*host}
	}
	repo, err := repoOrCwd(deps, *repoFlag)
	if err != nil {
		return err
	}
	settings, err := deps.Models.Load(ctx, repo)
	if err != nil {
		return err
	}
	result := showResult{OK: true, LocalPath: settings.LocalPath, GlobalPath: settings.GlobalPath, Hosts: map[string][]contract.Resolution{}}
	for _, h := range hosts {
		for _, role := range domain.Roles() {
			resolution, err := domain.Resolve(domain.ResolveInput{Host: h, Role: role, Local: settings.Local, Global: settings.Global})
			if err != nil {
				return err
			}
			result.Hosts[h] = append(result.Hosts[h], resolution)
		}
	}
	if *jsonOut {
		return jsonout.PrintTo(deps.Stdout, result)
	}
	fmt.Fprintf(deps.Stdout, "local:  %s\nglobal: %s\n", orNone(result.LocalPath), result.GlobalPath)
	for _, h := range hosts {
		for _, r := range result.Hosts[h] {
			fmt.Fprintf(deps.Stdout, "%s  %-15s  %s / %s  (%s, %s)\n", h, r.Role, orNone(r.Model), orNone(r.Effort), orNone(r.ModelSource), orNone(r.EffortSource))
		}
	}
	return nil
}

type target struct {
	scope, host string
	role        contract.Role
	repo        string
}

func parseTarget(fs *flag.FlagSet, args []string, deps Dependencies) (target, error) {
	scope := fs.String("scope", "", "global or local")
	host := fs.String("host", "", "claude or codex")
	role := fs.String("role", "", "role")
	repoFlag := fs.String("repo", "", "repository (defaults to cwd)")
	if err := parse(fs, args); err != nil {
		return target{}, err
	}
	if *scope != "global" && *scope != "local" {
		return target{}, UsageError{Message: fmt.Sprintf("--scope must be global or local, got %q", *scope)}
	}
	repo, err := repoOrCwd(deps, *repoFlag)
	if err != nil {
		return target{}, err
	}
	return target{scope: *scope, host: *host, role: contract.Role(*role), repo: repo}, nil
}

type setResult struct {
	OK       bool           `json:"ok"`
	Scope    string         `json:"scope"`
	Path     string         `json:"path"`
	Host     string         `json:"host"`
	Role     contract.Role  `json:"role"`
	Layer    contract.Layer `json:"layer"`
	Warnings []string       `json:"warnings"`
	Excluded *bool          `json:"excluded,omitempty"`
}

func runSet(ctx context.Context, deps Dependencies, args []string) error {
	fs := newFlagSet("set")
	model := fs.String("model", "", "model id or alias")
	effort := fs.String("effort", "", "reasoning effort")
	jsonOut := fs.Bool("json", false, "print JSON")
	t, err := parseTarget(fs, args, deps)
	if err != nil {
		return err
	}
	layer := contract.Layer{Model: strings.TrimSpace(*model), Effort: strings.TrimSpace(*effort)}
	var catalog []string
	if t.host == "codex" && deps.CodexCatalog != nil {
		catalog = deps.CodexCatalog()
	}
	warnings, err := domain.ValidateSetting(t.host, t.role, layer, catalog)
	if err != nil {
		return UsageError{Message: err.Error()}
	}
	settings, err := deps.Models.Load(ctx, t.repo)
	if err != nil {
		return err
	}
	cfg, path := scopeConfig(settings, t.scope)
	cfg = cloneConfig(cfg)
	section := hostSection(&cfg, t.host)
	entry := (*section)[t.role]
	if layer.Model != "" {
		entry.Model = layer.Model
	}
	if layer.Effort != "" {
		entry.Effort = layer.Effort
	}
	(*section)[t.role] = entry
	result := setResult{OK: true, Scope: t.scope, Path: path, Host: t.host, Role: t.role, Layer: entry, Warnings: append([]string{}, warnings...)}
	if err := writeScope(ctx, deps, t, path, cfg, &result.Excluded); err != nil {
		return err
	}
	if *jsonOut {
		return jsonout.PrintTo(deps.Stdout, result)
	}
	fmt.Fprintf(deps.Stdout, "%s %s.%s = %s / %s (%s)\n", t.scope, t.host, t.role, orNone(entry.Model), orNone(entry.Effort), path)
	for _, warning := range warnings {
		fmt.Fprintf(deps.Stdout, "warning: %s\n", warning)
	}
	return nil
}

type unsetResult struct {
	OK       bool          `json:"ok"`
	Scope    string        `json:"scope"`
	Path     string        `json:"path"`
	Host     string        `json:"host"`
	Role     contract.Role `json:"role"`
	Field    string        `json:"field,omitempty"`
	Removed  bool          `json:"removed"`
	Excluded *bool         `json:"excluded,omitempty"`
}

func runUnset(ctx context.Context, deps Dependencies, args []string) error {
	fs := newFlagSet("unset")
	field := fs.String("field", "", "model or effort (defaults to the whole role)")
	jsonOut := fs.Bool("json", false, "print JSON")
	t, err := parseTarget(fs, args, deps)
	if err != nil {
		return err
	}
	if *field != "" && *field != "model" && *field != "effort" {
		return UsageError{Message: fmt.Sprintf("--field must be model or effort, got %q", *field)}
	}
	// unset는 값을 받지 않으므로 host와 role만 검사한다. high는 두 host 모두 받는 effort다.
	if _, err := domain.ValidateSetting(t.host, t.role, contract.Layer{Effort: "high"}, nil); err != nil {
		return UsageError{Message: err.Error()}
	}
	settings, err := deps.Models.Load(ctx, t.repo)
	if err != nil {
		return err
	}
	cfg, path := scopeConfig(settings, t.scope)
	cfg = cloneConfig(cfg)
	section := hostSection(&cfg, t.host)
	entry, removed := removeField((*section)[t.role], *field)
	result := unsetResult{OK: true, Scope: t.scope, Path: path, Host: t.host, Role: t.role, Field: *field, Removed: removed}
	putLayer(section, t.role, entry)
	if result.Removed {
		if err := writeScope(ctx, deps, t, path, cfg, &result.Excluded); err != nil {
			return err
		}
	}
	if *jsonOut {
		return jsonout.PrintTo(deps.Stdout, result)
	}
	if !result.Removed {
		fmt.Fprintf(deps.Stdout, "%s %s.%s was not set (%s)\n", t.scope, t.host, t.role, path)
		return nil
	}
	fmt.Fprintf(deps.Stdout, "unset %s %s.%s %s(%s)\n", t.scope, t.host, t.role, strings.TrimSpace(*field+" "), path)
	return nil
}

type resolveResult struct {
	OK bool `json:"ok"`
	contract.Resolution
	Tier      string   `json:"tier,omitempty"`
	Round     int      `json:"round,omitempty"`
	Argv      []string `json:"argv"`
	AgentArgs []string `json:"agent_args,omitempty"`
}

func runResolve(ctx context.Context, deps Dependencies, args []string) error {
	fs := newFlagSet("resolve")
	host := fs.String("host", "", "claude, codex, omo, or omp")
	role := fs.String("role", "", "role")
	tier := fs.String("tier", "", "review tier (docs-only lowers a built-in review effort)")
	round := fs.Int("round", 0, "review round (3 and later escalate)")
	model := fs.String("model", "", "explicit model (flag layer)")
	effort := fs.String("effort", "", "explicit effort (flag layer)")
	agents := fs.Bool("agents", false, "also render the role-agent launch arguments")
	repoFlag := fs.String("repo", "", "repository (defaults to cwd)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := parse(fs, args); err != nil {
		return err
	}
	if *round < 0 {
		return UsageError{Message: "--round must not be negative"}
	}
	input := domain.ResolveInput{Host: *host, Role: contract.Role(*role), Tier: *tier, Round: *round, Flag: contract.Layer{Model: strings.TrimSpace(*model), Effort: strings.TrimSpace(*effort)}}
	if _, err := domain.Resolve(input); err != nil {
		return UsageError{Message: err.Error()}
	}
	repo, err := repoOrCwd(deps, *repoFlag)
	if err != nil {
		return err
	}
	resolution, err := deps.Models.Resolve(ctx, repo, input)
	if err != nil {
		return err
	}
	result := resolveResult{OK: true, Resolution: resolution, Tier: *tier, Round: *round, Argv: deps.PrintArgv(resolution.Host, resolution.Model, resolution.Effort)}
	if result.Argv == nil {
		result.Argv = []string{}
	}
	if *agents {
		if result.AgentArgs, err = deps.RoleAgentArgs(ctx, resolution.Host, repo); err != nil {
			return err
		}
	}
	if *jsonOut {
		return jsonout.PrintTo(deps.Stdout, result)
	}
	fmt.Fprintf(deps.Stdout, "%s %s: %s / %s (%s, %s)\n", resolution.Host, resolution.Role, orNone(resolution.Model), orNone(resolution.Effort), orNone(resolution.ModelSource), orNone(resolution.EffortSource))
	return nil
}

// putLayer stores entry, dropping an empty entry and an empty host section
// so the file never keeps empty objects.
func putLayer(section *map[contract.Role]contract.Layer, role contract.Role, entry contract.Layer) {
	if entry == (contract.Layer{}) {
		delete(*section, role)
	} else {
		(*section)[role] = entry
	}
	if len(*section) == 0 {
		*section = nil
	}
}

// removeField clears one field, or the whole layer when field is empty, and
// reports whether anything was set.
func removeField(entry contract.Layer, field string) (contract.Layer, bool) {
	switch field {
	case "model":
		return contract.Layer{Effort: entry.Effort}, entry.Model != ""
	case "effort":
		return contract.Layer{Model: entry.Model}, entry.Effort != ""
	default:
		return contract.Layer{}, entry != (contract.Layer{})
	}
}

func scopeConfig(settings agentmodelapp.Settings, scope string) (contract.Config, string) {
	if scope == "local" {
		return settings.Local, settings.LocalPath
	}
	return settings.Global, settings.GlobalPath
}

func writeScope(ctx context.Context, deps Dependencies, t target, path string, cfg contract.Config, excluded **bool) error {
	cfg.Version = contract.ConfigVersion
	if t.scope == "global" {
		return deps.WriteGlobal(path, cfg)
	}
	if path == "" {
		return errors.New("local settings need a git repository")
	}
	added, err := deps.WriteLocal(ctx, t.repo, cfg)
	if err != nil {
		return err
	}
	*excluded = &added
	return nil
}

func hostSection(cfg *contract.Config, host string) *map[contract.Role]contract.Layer {
	section := &cfg.Claude
	if host == "codex" {
		section = &cfg.Codex
	}
	if *section == nil {
		*section = map[contract.Role]contract.Layer{}
	}
	return section
}

func cloneConfig(cfg contract.Config) contract.Config {
	cfg.Claude = maps.Clone(cfg.Claude)
	cfg.Codex = maps.Clone(cfg.Codex)
	return cfg
}

func orNone(value string) string {
	if value == "" {
		return "-"
	}
	return value
}
