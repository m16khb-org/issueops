package parallelisolation

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	verifycontract "issueops/internal/contract/selfverify"
	verifydomain "issueops/internal/domain/selfverify"
)

const aggregateOutputBudgetBytes = 8 * 1024

func parallelIsolationResult(started time.Time, workers int, probes []parallelIsolationProbe) verifycontract.StepResult {
	sort.Slice(probes, func(i, j int) bool { return probes[i].Worker < probes[j].Worker })
	stdoutText, stdoutTruncated, stdoutOriginalBytes := parallelIsolationOutput(workers, probes)
	errs := parallelIsolationErrors(probes)
	if len(errs) > 0 {
		return verifycontract.StepResult{Label: "parallel isolation", OK: false, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated, Error: strings.Join(errs, "; ")}
	}
	return verifycontract.StepResult{Label: "parallel isolation", Command: strings.Join(parallelIsolationCommands(probes), " && "), OK: true, DurationMS: time.Since(started).Milliseconds(), Stdout: stdoutText, StdoutBytes: stdoutOriginalBytes, StdoutTruncated: stdoutTruncated}
}

func parallelIsolationOutput(workers int, probes []parallelIsolationProbe) (string, bool, int) {
	stdoutBytes, _ := json.MarshalIndent(map[string]any{
		"workers": workers,
		"probes":  probes,
	}, "", "  ")
	return verifydomain.TailWithBudget(string(stdoutBytes), aggregateOutputBudgetBytes)
}

func parallelIsolationErrors(probes []parallelIsolationProbe) []string {
	errs := []string{}
	seenPaths := map[string]string{}
	for _, probe := range probes {
		if probe.Error != "" {
			errs = append(errs, fmt.Sprintf("worker %d: %s", probe.Worker, probe.Error))
		}
		for _, field := range []struct{ name, value string }{
			{"temp_root", probe.TempRoot},
			{"state_dir", probe.StateDir},
			{"artifact_path", probe.ArtifactPath},
		} {
			if strings.TrimSpace(field.value) == "" {
				errs = append(errs, fmt.Sprintf("worker %d has empty %s", probe.Worker, field.name))
				continue
			}
			if previous, ok := seenPaths[field.value]; ok {
				errs = append(errs, fmt.Sprintf("path collision: %s reused by %s and worker %d %s", field.value, previous, probe.Worker, field.name))
				continue
			}
			seenPaths[field.value] = fmt.Sprintf("worker %d %s", probe.Worker, field.name)
		}
	}
	return errs
}

func parallelIsolationCommands(probes []parallelIsolationProbe) []string {
	commands := []string{}
	for _, probe := range probes {
		commands = append(commands, probe.Commands...)
	}
	return commands
}
