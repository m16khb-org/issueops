package basiccli

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"issueops/cmd/issueops/cliflag"
	doctorcontract "issueops/internal/contract/doctor"
	inspectcontract "issueops/internal/contract/inspect"
	"issueops/internal/domain/operationalhealth"
)

type doctorRepeatedFlag []string

func (values *doctorRepeatedFlag) String() string {
	return fmt.Sprint([]string(*values))
}

func (values *doctorRepeatedFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

func (command Command) RunInspect(args []string) error {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "print JSON")
	repo := fs.String("repo", "", "target repo/workspace")
	hostReceipts := fs.String("host-receipts", "", "host receipt JSON file (schema_version 1) for discovered/connected/protocol observations")
	positional, err := cliflag.ParseOptionalPositional(fs, args)
	if err != nil {
		return err
	}
	if *repo == "" && positional != "" {
		*repo = positional
	}
	info := command.InspectHarness(*repo, inspectcontract.Options{HostReceipts: *hostReceipts})
	if *jsonOut {
		return printJSON(info)
	}
	fmt.Printf("issueops root: %s\n", info.IssueOpsRoot)
	fmt.Printf("target repo: %s\n", info.TargetRepo)
	fmt.Printf("skills: %d\n", len(info.Skills))
	for _, s := range info.Skills {
		fmt.Printf("- %s (%s)\n", s.Name, s.Path)
	}
	fmt.Printf("codex skill installed: %v\n", info.Integration.CodexSkillInstalled)
	fmt.Printf("claude skill installed: %v\n", info.Integration.ClaudeSkillInstalled)
	fmt.Printf("project Claude MCP config: %v\n", info.Integration.ProjectClaudeMCPConfig)
	for _, host := range info.Integration.Hosts {
		fmt.Printf("host %s (%s): installed=%s linked=%s configured=%s discovered=%s connected=%s protocol=%s\n", host.Host, host.Transport,
			host.Installed.Status, host.Linked.Status, host.Configured.Status, host.Discovered.Status, host.Connected.Status, host.Protocol.Status)
	}
	return nil
}

func (command Doctor) Run(args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	staticOnly := fs.Bool("static-only", false, "skip live operational, pipe, and MCP probes")
	var preserveCycles doctorRepeatedFlag
	fs.Var(&preserveCycles, "preserve-cycle", "preserve one exact IssueOps cycle for this invocation (repeatable)")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage: issueops doctor [--repo PATH] [--static-only] [--preserve-cycle ID]... [--json]")
		fs.PrintDefaults()
	}
	positional, err := cliflag.ParseOptionalPositional(fs, args)
	if err != nil {
		return err
	}
	if positional != "" {
		*repo = positional
	}
	cycleIDs, err := normalizeDoctorPreserve(preserveCycles, "--preserve-cycle")
	if err != nil {
		return err
	}
	root, err := command.NormalizeRepoRoot(*repo)
	if err != nil {
		return err
	}
	var snapshot *operationalhealth.Snapshot
	if !*staticOnly {
		observed := command.CollectOperationalHealth(context.Background(), root)
		snapshot = &observed
	}
	result, err := command.Service.Run(doctorcontract.HarnessDoctorRequest{
		RepoRoot:            root,
		IssueOpsRoot:        command.IssueOpsRoot,
		Home:                command.Home,
		Version:             command.Version,
		StaticOnly:          *staticOnly,
		OperationalSnapshot: snapshot,
		OperationalOptions: operationalhealth.Options{
			Now:              command.Now().UTC(),
			PreserveCycleIDs: cycleIDs,
		},
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	if result.Healthy {
		fmt.Printf("issueops doctor healthy: %s\n", result.RepoRoot)
		return nil
	}
	fmt.Printf("issueops doctor found %d issues for %s\n", len(result.Issues), result.RepoRoot)
	for _, issue := range result.Issues {
		fmt.Printf("%s %s %s\n", issue.Severity, issue.Code, issue.Summary)
		if issue.Fix != nil && issue.Fix.Command != "" {
			fmt.Printf("  fix: %s\n", issue.Fix.Command)
		}
	}
	return nil
}

func normalizeDoctorPreserve(values []string, flagName string) ([]string, error) {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			return nil, fmt.Errorf("%s requires a non-empty value", flagName)
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result, nil
}
