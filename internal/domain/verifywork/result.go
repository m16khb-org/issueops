package verifywork

import (
	"fmt"
	verifyworkcontract "issueops/internal/contract/verifywork"
	projectdocdomain "issueops/internal/domain/projectdoc"
	"strings"
)

type Observation struct {
	GitFailed      bool
	GitError       string
	PreflightOK    bool
	GuardOK        bool
	GuardMode      string
	GuardFileCount int
	CommandPresent bool
	CommandOK      bool
	Argv           []string
	Signals        projectdocdomain.ProjectSignals
}

type Decision struct {
	OK                bool
	Evidence          []string
	EvidenceMatrix    []verifyworkcontract.EvidenceItem
	SuggestedCommands []verifyworkcontract.SuggestedCommand
	Warnings          []string
}

func Evaluate(facts Observation) Decision {
	warnings := []string{}
	evidence := []string{}
	evidenceMatrix := []verifyworkcontract.EvidenceItem{}
	if facts.GitFailed {
		warnings = append(warnings, "git status: "+facts.GitError)
	}
	if facts.PreflightOK {
		evidence = append(evidence, "git preflight completed")
	} else {
		warnings = append(warnings, "git preflight reported issues")
	}
	evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItem("git_preflight", facts.PreflightOK, "git repository preflight completed"))
	if facts.GuardOK {
		evidence = append(evidence, fmt.Sprintf("guard check passed (%s, %d files)", facts.GuardMode, facts.GuardFileCount))
	} else {
		warnings = append(warnings, "guard check has blocking findings")
	}
	evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItem("guard_check", facts.GuardOK, fmt.Sprintf("guard check completed in %s mode for %d file(s)", facts.GuardMode, facts.GuardFileCount)))
	if facts.CommandPresent {
		if facts.CommandOK {
			evidence = append(evidence, "read-only verification command passed")
		} else {
			warnings = append(warnings, "read-only verification command failed or was denied")
		}
		evidenceMatrix = append(evidenceMatrix, verifyWorkEvidenceItemWithCommand("read_only_command", facts.CommandOK, "read-only verification command completed", strings.Join(facts.Argv, " ")))
	} else {
		evidenceMatrix = append(evidenceMatrix, verifyworkcontract.EvidenceItem{Name: "read_only_command", OK: true, Status: "skipped", Summary: "no read-only verification command provided"})
	}
	ok := facts.PreflightOK && facts.GuardOK && len(warnings) == 0
	if facts.CommandPresent {
		ok = ok && facts.CommandOK
	}
	return Decision{OK: ok, Evidence: evidence, EvidenceMatrix: evidenceMatrix, SuggestedCommands: suggestedCommands(facts.Signals), Warnings: warnings}
}

func verifyWorkEvidenceItem(name string, ok bool, summary string) verifyworkcontract.EvidenceItem {
	return verifyWorkEvidenceItemWithCommand(name, ok, summary, "")
}

func verifyWorkEvidenceItemWithCommand(name string, ok bool, summary string, command string) verifyworkcontract.EvidenceItem {
	status := "failed"
	if ok {
		status = "passed"
	}
	return verifyworkcontract.EvidenceItem{Name: name, OK: ok, Status: status, Summary: summary, Command: command}
}

func suggestedCommands(signals projectdocdomain.ProjectSignals) []verifyworkcontract.SuggestedCommand {
	out := []verifyworkcontract.SuggestedCommand{}
	add := func(kind string, commands []projectdocdomain.EvidenceCommand) {
		for i, command := range commands {
			fields := strings.Fields(command.Command)
			if len(fields) == 0 {
				continue
			}
			reason := fmt.Sprintf("%s command inferred from %s (confidence=%s)", kind, strings.Join(command.Evidence, ","), command.Confidence)
			out = append(out, verifyworkcontract.SuggestedCommand{Name: fmt.Sprintf("%s_%d", kind, i+1), Command: fields, Reason: reason})
		}
	}
	add("test", signals.TestCommands)
	add("build", signals.BuildCommands)
	add("lint", signals.LintCommands)
	return out
}
