package issueopscli

import (
	"context"
	commandparsecontract "issueops/internal/contract/commandparse"
	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/commandparse"
	"strings"
)

func (cli command) prepareGeneratedCommandInvocation(args []string, deps Dependencies) ([]string, bool, error) {
	clean, expected, present, err := commandparsecontract.ConsumeGeneratedCommandProvenance(args)
	if err != nil || !present {
		return clean, false, err
	}
	id := issueOpsCommandID(clean)
	if id == "" {
		return nil, true, &commandparsecontract.GeneratedCommandProvenanceError{
			Code: "generated_command_provenance_invalid", Message: "generated command provenance requires an IssueOps id",
		}
	}
	record, err := cli.Runtime.ReadIssueOps(cli.Runtime.IssueOpsStateRoot(), id)
	if err != nil {
		return nil, true, err
	}
	authority := record
	if authority.Execution == nil {
		authority, err = cli.generatedDelegatedBootstrapAuthority(clean, record)
		if err != nil {
			return nil, true, err
		}
	}
	if deps.Provenance == nil {
		return nil, true, commandparsecontract.NewGeneratedCommandProvenanceObservationError(nil)
	}
	receipt, err := deps.Provenance.Observe(context.Background())
	if err != nil {
		return nil, true, commandparsecontract.NewGeneratedCommandProvenanceObservationError(err)
	}
	observed := commandparsecontract.GeneratedCommandProvenance{
		ExecutablePath:   receipt.ExecutablePath,
		ExecutableSHA256: receipt.ExecutableSHA256,
		LeaseGeneration:  authority.Execution.Lease.Generation,
	}
	if err := commandparse.ValidateGeneratedCommandInvocation(expected, observed, authority.Execution.Lease.Generation); err != nil {
		return nil, true, err
	}
	return clean, true, nil
}

func generatedProvenanceInvalid(message string) (issueopscontract.IssueOpsRecord, error) {
	return issueopscontract.IssueOpsRecord{}, &commandparsecontract.GeneratedCommandProvenanceError{
		Code: "generated_command_provenance_invalid", Message: message,
	}
}

func (cli command) generatedDelegatedBootstrapAuthority(args []string, child issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsRecord, error) {
	command, ok := commandparse.ParseExactIssueOpsArgs(args)
	if !ok || command.Path != "branch prepare" && command.Path != "execution prepare" || child.Delegation == nil {
		return generatedProvenanceInvalid("generated command provenance requires IssueOps execution state")
	}
	values, booleans, repeatable, ok := commandparse.IssueOpsCommandSpec(command.Path)
	if !ok {
		return generatedProvenanceInvalid("generated delegated child bootstrap command is unsupported")
	}
	flags, ok := commandparse.ExactFlags(command, values, booleans, repeatable)
	if !ok {
		return generatedProvenanceInvalid("generated delegated child bootstrap command is malformed")
	}
	parentID := strings.TrimSpace(child.Delegation.ParentCycleID)
	parent, err := cli.Runtime.ReadIssueOps(cli.Runtime.IssueOpsStateRoot(), parentID)
	if err != nil || parent.Execution == nil || parent.Execution.Lease.Status != issueopscontract.LeaseStatusActive ||
		!generatedParentReferencesChild(parent, child.ID) {
		return generatedProvenanceInvalid("generated delegated child bootstrap requires an active referenced parent execution")
	}
	switch command.Path {
	case "branch prepare":
		if !generatedBranchPrepareMatchesParent(flags, child, parent) {
			return generatedProvenanceInvalid("generated delegated child branch preparation does not match parent topology")
		}
	case "execution prepare":
		if !generatedExecutionPrepareMatchesParent(child, parent) {
			return generatedProvenanceInvalid("generated delegated child execution preparation does not match parent topology")
		}
	}
	return parent, nil
}

// generatedBranchPrepareMatchesParent는 branch prepare 플래그가 자식 브랜치와 부모 토폴로지에 일치하는지 확인한다.
func generatedBranchPrepareMatchesParent(flags map[string][]string, child, parent issueopscontract.IssueOpsRecord) bool {
	branch, branchOK := oneGeneratedFlag(flags, "--branch")
	baseBranch, baseOK := oneGeneratedFlag(flags, "--base-branch")
	parentWorktree, worktreeOK := oneGeneratedFlag(flags, "--parent-worktree")
	return branchOK && baseOK && worktreeOK &&
		strings.TrimSpace(branch) == strings.TrimSpace(child.Branch) &&
		strings.TrimSpace(baseBranch) == strings.TrimSpace(parent.Branch) &&
		sameExistingIssueOpsPath(parentWorktree, parent.Execution.Workspace.Root)
}

// generatedExecutionPrepareMatchesParent는 기록된 브랜치 준비 결과가 부모 토폴로지에 일치하는지 확인한다.
func generatedExecutionPrepareMatchesParent(child, parent issueopscontract.IssueOpsRecord) bool {
	prepared := child.BranchPrepare
	return prepared != nil &&
		strings.TrimSpace(prepared.Branch) == strings.TrimSpace(child.Branch) &&
		strings.TrimSpace(prepared.BaseBranch) == strings.TrimSpace(parent.Branch) &&
		sameExistingIssueOpsPath(prepared.ParentWorktree, parent.Execution.Workspace.Root)
}

func generatedParentReferencesChild(parent issueopscontract.IssueOpsRecord, childID string) bool {
	for _, child := range parent.ChildCycles {
		if strings.TrimSpace(child.CycleID) == strings.TrimSpace(childID) {
			return true
		}
	}
	return false
}

func oneGeneratedFlag(flags map[string][]string, name string) (string, bool) {
	values := flags[name]
	return strings.TrimSpace(firstGeneratedValue(values)), len(values) == 1
}

func firstGeneratedValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func issueOpsCommandID(args []string) string {
	idFlag := "--id"
	if len(args) >= 2 && args[0] == "child" {
		idFlag = "--parent"
	}
	for index := 0; index < len(args); index++ {
		if args[index] == idFlag && index+1 < len(args) {
			return strings.TrimSpace(args[index+1])
		}
		if strings.HasPrefix(args[index], idFlag+"=") {
			return strings.TrimSpace(strings.TrimPrefix(args[index], idFlag+"="))
		}
	}
	return ""
}

func issueOpsJSONRequested(args []string) bool {
	for _, arg := range args {
		if arg == "--json" {
			return true
		}
	}
	return false
}
