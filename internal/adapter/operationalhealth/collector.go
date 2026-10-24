package operationalhealth

import (
	"bytes"
	"context"
	"fmt"
	"issueops/internal/adapter/outbound/gitbin"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	corehealth "issueops/internal/domain/operationalhealth"
)

type GitRunner interface {
	Run(context.Context, string, ...string) ([]byte, error)
}

type NativeProcessInspector func(issueopscontract.NativeProcessReceipt) (string, issueopscontract.NativeProcessReceipt, error)

const gitInventoryCommandTimeout = 15 * time.Second

type ExecGitRunner struct {
	timeout time.Duration
}

func (runner ExecGitRunner) Run(ctx context.Context, repo string, args ...string) ([]byte, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("git arguments are required")
	}
	timeout := runner.timeout
	if timeout <= 0 {
		timeout = gitInventoryCommandTimeout
	}
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(commandCtx, gitbin.Path(), args...)
	command.Dir = repo
	command.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GCM_INTERACTIVE=Never", "SSH_ASKPASS_REQUIRE=never")
	output, err := command.Output()
	if commandCtx.Err() != nil {
		return nil, fmt.Errorf("git inventory command: %w", commandCtx.Err())
	}
	return output, err
}

func canonicalInventoryPath(path string) string {
	abs := strings.TrimSpace(path)
	if abs != "" && !filepath.IsAbs(abs) {
		if resolved, err := filepath.Abs(abs); err == nil {
			abs = resolved
		}
	}
	if abs != "" {
		abs = filepath.Clean(abs)
	}
	if abs == "" {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return filepath.Clean(resolved)
	}
	original := abs
	missing := make([]string, 0, 2)
	for {
		parent := filepath.Dir(abs)
		if parent == abs {
			return original
		}
		missing = append([]string{filepath.Base(abs)}, missing...)
		if resolved, err := filepath.EvalSymlinks(parent); err == nil {
			parts := append([]string{filepath.Clean(resolved)}, missing...)
			return filepath.Join(parts...)
		}
		abs = parent
	}
}

type Collector struct {
	IssueOps             IssueOpsReader
	Git                  GitRunner
	InspectNativeProcess NativeProcessInspector
}

func (collector Collector) Collect(ctx context.Context, repo string) corehealth.Snapshot {
	snapshot := corehealth.Snapshot{RepoRoot: canonicalInventoryPath(repo)}
	if snapshot.RepoRoot == "" {
		addProblem(&snapshot, "repo", "repo_invalid", "requested repository path is empty")
		return snapshot
	}
	collector.collectIssueOps(&snapshot)
	collector.collectGit(ctx, &snapshot, true)
	sortSnapshot(&snapshot)
	return snapshot
}

// CollectLocal refreshes strict persisted ownership and local Git facts only.
// It performs no provider or remote-ref requests and takes no SQLite span.
func (collector Collector) CollectLocal(ctx context.Context, repo string) corehealth.Snapshot {
	snapshot := corehealth.Snapshot{RepoRoot: canonicalInventoryPath(repo)}
	collector.collectIssueOps(&snapshot)
	collector.collectGit(ctx, &snapshot, false)
	sortSnapshot(&snapshot)
	return snapshot
}

func (collector Collector) collectGit(ctx context.Context, snapshot *corehealth.Snapshot, includeRemote bool) {
	if collector.Git == nil {
		addProblem(snapshot, "git", "git_runner_missing", "Git inventory reader is unavailable")
		return
	}
	commands := []struct {
		source string
		code   string
		args   []string
		apply  func([]byte) error
	}{
		{
			source: "git_branch", code: "git_symbolic_ref_failed",
			args: []string{"symbolic-ref", "--quiet", "--short", "HEAD"},
			apply: func(output []byte) error {
				snapshot.CanonicalBranch = strings.TrimSpace(string(output))
				if snapshot.CanonicalBranch == "" {
					return fmt.Errorf("empty branch")
				}
				return nil
			},
		},
		{
			source: "git_head", code: "git_head_failed",
			args: []string{"rev-parse", "--verify", "HEAD"},
			apply: func(output []byte) error {
				snapshot.SourceHead = strings.TrimSpace(string(output))
				if !validOID(snapshot.SourceHead) {
					return fmt.Errorf("invalid head")
				}
				return nil
			},
		},
		{
			source: "git_status", code: "git_status_failed",
			args: []string{"status", "--porcelain=v1", "-z"},
			apply: func(output []byte) error {
				snapshot.SourceClean = len(output) == 0
				return nil
			},
		},
		{
			source: "git_worktrees", code: "git_worktrees_failed",
			args: []string{"worktree", "list", "--porcelain", "-z"},
			apply: func(output []byte) error {
				values, err := parseWorktrees(output, snapshot.RepoRoot)
				snapshot.GitWorktrees = values
				return err
			},
		},
		{
			source: "git_local_refs", code: "git_local_refs_failed",
			args: []string{"for-each-ref", "--format=%(refname)%00%(objectname)%00", "refs/heads"},
			apply: func(output []byte) error {
				values, err := parseLocalRefs(output)
				snapshot.LocalRefs = values
				return err
			},
		},
		{
			source: "git_remote_refs", code: "git_remote_refs_failed",
			args: []string{"ls-remote", "--heads", "origin"},
			apply: func(output []byte) error {
				values, err := parseRemoteRefs(output)
				snapshot.RemoteRefs = values
				return err
			},
		},
	}
	for _, command := range commands {
		if !includeRemote && command.source == "git_remote_refs" {
			continue
		}
		output, err := collector.Git.Run(ctx, snapshot.RepoRoot, command.args...)
		if err != nil {
			addProblem(snapshot, command.source, command.code, command.source+" inventory failed")
			continue
		}
		if err := command.apply(output); err != nil {
			addProblem(snapshot, command.source, command.code, command.source+" inventory is malformed")
		}
	}
	for index := range snapshot.GitWorktrees {
		if snapshot.GitWorktrees[index].Canonical {
			snapshot.GitWorktrees[index].Clean = snapshot.SourceClean
			if snapshot.SourceHead != "" && snapshot.GitWorktrees[index].Head != snapshot.SourceHead {
				addProblem(snapshot, "git_worktrees", "git_source_identity_mismatch", "canonical Git worktree head does not match source HEAD")
			}
		}
	}
}

func (collector Collector) collectIssueOps(snapshot *corehealth.Snapshot) {
	stateRoot := collector.IssueOps.StateRoot
	records := []issueopscontract.IssueOpsRecord{}
	unreadable := []string{}
	err := collector.IssueOps.Scan(stateRoot, func(id string, record issueopscontract.IssueOpsRecord, err error) {
		if err != nil {
			unreadable = append(unreadable, id)
			return
		}
		records = append(records, record)
	})
	if err != nil {
		addProblem(snapshot, "issueops", "issueops_list_failed", "IssueOps ID inventory failed")
		return
	}
	for _, id := range unreadable {
		addProblem(snapshot, "issueops_record", "issueops_read_failed", "could not read IssueOps record "+strings.TrimSpace(id))
	}
	for _, record := range records {
		cycle, problems := cycleFromRecord(record, collector.InspectNativeProcess)
		snapshot.Cycles = append(snapshot.Cycles, cycle)
		snapshot.InventoryProblems = append(snapshot.InventoryProblems, problems...)
	}
	indexes, err := collector.IssueOps.ListLeaseHolders(stateRoot)
	if err != nil {
		addProblem(snapshot, "issueops_lease_holder", "issueops_lease_holder_list_failed", "IssueOps active lease-holder index inventory failed")
	} else {
		for _, index := range indexes {
			snapshot.LeaseHolderIndexes = append(snapshot.LeaseHolderIndexes, operationalhealthcontract.LeaseHolderIndex{
				Key: index.Key, LifecycleID: index.LifecycleID, Generation: index.Generation,
				Host: index.Host, SessionID: index.SessionID, AgentID: index.AgentID,
			})
		}
	}
}

func cycleFromRecord(record issueopscontract.IssueOpsRecord, inspect NativeProcessInspector) (operationalhealthcontract.Cycle, []operationalhealthcontract.InventoryProblem) {
	return cycleFromRecordAt(record, inspect, time.Now().UTC())
}

func cycleFromRecordAt(record issueopscontract.IssueOpsRecord, inspect NativeProcessInspector, now time.Time) (operationalhealthcontract.Cycle, []operationalhealthcontract.InventoryProblem) {
	cycle := operationalhealthcontract.Cycle{
		ID: strings.TrimSpace(record.ID), Repo: canonicalInventoryPath(record.Repo), Branch: strings.TrimSpace(record.Branch),
		Phase:                     string(record.Phase),
		ExecutionFailurePresent:   record.Execution != nil && record.Execution.Failure != nil,
		CleanupFailurePresent:     record.CleanupFinishFailure != nil || record.CleanupAbandonFailure != nil,
		IssueCreateFailurePresent: issueCreateIntentNeedsReconciliationAt(record.IssueCreateIntent, now),
	}
	var problems []operationalhealthcontract.InventoryProblem
	worktreeConflict := false
	mergeWorktreePath := func(raw string) {
		path := canonicalInventoryPath(raw)
		if path == "" {
			return
		}
		if cycle.WorktreePath == "" {
			cycle.WorktreePath = path
			return
		}
		if cycle.WorktreePath != path && !worktreeConflict {
			problems = append(problems, operationalhealthcontract.InventoryProblem{Source: "issueops_record", Code: "issueops_worktree_identity_mismatch", Detail: "IssueOps record " + cycle.ID + " contains conflicting worktree paths"})
			worktreeConflict = true
		}
	}
	mergeWorktreePath(record.WorktreePath)
	if record.Execution == nil {
		return cycle, problems
	}
	execution := record.Execution
	cycle.LeaseStatus = string(execution.Lease.Status)
	cycle.Generation = execution.Lease.Generation
	cycle.CompletionPresent = execution.Completion != nil
	mergeWorktreePath(execution.Workspace.Root)
	if execution.Lease.Holder != nil {
		cycle.HolderHost = strings.TrimSpace(execution.Lease.Holder.Host)
		cycle.HolderSessionID = strings.TrimSpace(execution.Lease.Holder.SessionID)
		cycle.HolderAgentID = strings.TrimSpace(execution.Lease.Holder.AgentID)
		if execution.Lease.Holder.SessionProcess != nil {
			receipt := *execution.Lease.Holder.SessionProcess
			cycle.HolderPID = receipt.PID
			cycle.HolderStartedAt = strings.TrimSpace(receipt.StartedAt)
			cycle.HolderExecutable = strings.TrimSpace(receipt.Executable)
			if inspect != nil && execution.Lease.Status == issueopscontract.LeaseStatusActive {
				status, _, err := inspect(receipt)
				cycle.HolderProcessStatus = strings.TrimSpace(status)
				if err != nil {
					cycle.HolderProcessStatus = operationalhealthcontract.ProcessStatusUnknown
					problems = append(problems, operationalhealthcontract.InventoryProblem{Source: "issueops_process", Code: "issueops_process_probe_failed", Detail: "IssueOps record " + cycle.ID + " native process identity could not be observed"})
				}
			}
		}
	}
	return cycle, problems
}

func issueCreateIntentNeedsReconciliationAt(intent *issueopscontract.IssueOpsIssueCreateIntent, now time.Time) bool {
	if intent == nil {
		return false
	}
	return corehealth.IssueCreateIntentNeedsReconciliationAt(intent.Status, intent.UpdatedAt, intent.StartedAt, now)
}

func parseWorktrees(output []byte, repo string) ([]operationalhealthcontract.GitWorktree, error) {
	var result []operationalhealthcontract.GitWorktree
	current := operationalhealthcontract.GitWorktree{}
	flush := func() error {
		if current.Path == "" {
			if current.Head == "" && current.Branch == "" {
				return nil
			}
			return fmt.Errorf("worktree path missing")
		}
		if current.Head == "" || !validOID(current.Head) {
			return fmt.Errorf("worktree head invalid")
		}
		current.Canonical = current.Path == repo
		result = append(result, current)
		current = operationalhealthcontract.GitWorktree{}
		return nil
	}
	for raw := range bytes.SplitSeq(output, []byte{0}) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			if err := flush(); err != nil {
				return result, err
			}
			continue
		}
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Path = canonicalInventoryPath(value)
		case "HEAD":
			current.Head = strings.TrimSpace(value)
		case "branch":
			current.Branch = strings.TrimPrefix(strings.TrimSpace(value), "refs/heads/")
		}
	}
	if err := flush(); err != nil {
		return result, err
	}
	return result, nil
}

func parseLocalRefs(output []byte) ([]operationalhealthcontract.GitRef, error) {
	var fields []string
	for raw := range bytes.SplitSeq(output, []byte{0}) {
		if value := strings.TrimSpace(string(raw)); value != "" {
			fields = append(fields, value)
		}
	}
	if len(fields)%2 != 0 {
		return nil, fmt.Errorf("local ref tuple incomplete")
	}
	result := make([]operationalhealthcontract.GitRef, 0, len(fields)/2)
	for index := 0; index < len(fields); index += 2 {
		name, oid := fields[index], fields[index+1]
		if !strings.HasPrefix(name, "refs/heads/") || !validOID(oid) {
			return result, fmt.Errorf("local ref identity invalid")
		}
		result = append(result, operationalhealthcontract.GitRef{Name: name, Branch: strings.TrimPrefix(name, "refs/heads/"), OID: oid, Location: "local"})
	}
	return result, nil
}

func parseRemoteRefs(output []byte) ([]operationalhealthcontract.GitRef, error) {
	var result []operationalhealthcontract.GitRef
	for raw := range bytes.SplitSeq(output, []byte{'\n'}) {
		line := strings.TrimSpace(string(raw))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 || !validOID(fields[0]) || !strings.HasPrefix(fields[1], "refs/heads/") {
			return result, fmt.Errorf("remote ref identity invalid")
		}
		result = append(result, operationalhealthcontract.GitRef{Name: fields[1], Branch: strings.TrimPrefix(fields[1], "refs/heads/"), OID: fields[0], Location: "remote"})
	}
	return result, nil
}

func validOID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}

func addProblem(snapshot *corehealth.Snapshot, source, code, detail string) {
	snapshot.InventoryProblems = append(snapshot.InventoryProblems, operationalhealthcontract.InventoryProblem{Source: source, Code: code, Detail: detail})
}

func sortSnapshot(snapshot *corehealth.Snapshot) {
	sort.Slice(snapshot.InventoryProblems, func(i, j int) bool {
		left, right := snapshot.InventoryProblems[i], snapshot.InventoryProblems[j]
		return orderedBefore([]string{left.Source, left.Code, left.Detail}, []string{right.Source, right.Code, right.Detail})
	})
	sort.Slice(snapshot.Cycles, func(i, j int) bool {
		left, right := snapshot.Cycles[i], snapshot.Cycles[j]
		return orderedBefore(
			[]string{left.ID, left.Repo, left.Branch, left.Phase, left.LeaseStatus, strconv.FormatUint(left.Generation, 10), left.HolderHost, left.HolderSessionID, left.HolderAgentID, strconv.Itoa(left.HolderPID), left.HolderStartedAt, left.HolderExecutable, left.HolderProcessStatus, strconv.FormatBool(left.CompletionPresent), left.WorktreePath},
			[]string{right.ID, right.Repo, right.Branch, right.Phase, right.LeaseStatus, strconv.FormatUint(right.Generation, 10), right.HolderHost, right.HolderSessionID, right.HolderAgentID, strconv.Itoa(right.HolderPID), right.HolderStartedAt, right.HolderExecutable, right.HolderProcessStatus, strconv.FormatBool(right.CompletionPresent), right.WorktreePath},
		)
	})
	sort.Slice(snapshot.LeaseHolderIndexes, func(i, j int) bool {
		left, right := snapshot.LeaseHolderIndexes[i], snapshot.LeaseHolderIndexes[j]
		return orderedBefore(
			[]string{left.Key, left.LifecycleID, strconv.FormatUint(left.Generation, 10), left.Host, left.SessionID, left.AgentID},
			[]string{right.Key, right.LifecycleID, strconv.FormatUint(right.Generation, 10), right.Host, right.SessionID, right.AgentID},
		)
	})
	sort.Slice(snapshot.GitWorktrees, func(i, j int) bool {
		left, right := snapshot.GitWorktrees[i], snapshot.GitWorktrees[j]
		return orderedBefore(
			[]string{left.Path, left.Branch, left.Head, strconv.FormatBool(left.Clean), strconv.FormatBool(left.Canonical)},
			[]string{right.Path, right.Branch, right.Head, strconv.FormatBool(right.Clean), strconv.FormatBool(right.Canonical)},
		)
	})
	sortRefs(snapshot.LocalRefs)
	sortRefs(snapshot.RemoteRefs)
	sort.Slice(snapshot.StateArtifacts, func(i, j int) bool {
		left, right := snapshot.StateArtifacts[i], snapshot.StateArtifacts[j]
		return orderedBefore([]string{left.Path, left.Code}, []string{right.Path, right.Code})
	})
}

func sortRefs(values []operationalhealthcontract.GitRef) {
	sort.Slice(values, func(i, j int) bool {
		left, right := values[i], values[j]
		return orderedBefore([]string{left.Name, left.Branch, left.OID, left.Location}, []string{right.Name, right.Branch, right.OID, right.Location})
	})
}

func orderedBefore(left, right []string) bool {
	for index := 0; index < len(left) && index < len(right); index++ {
		if left[index] != right[index] {
			return left[index] < right[index]
		}
	}
	return len(left) < len(right)
}
