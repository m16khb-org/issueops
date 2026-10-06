package state

import (
	"sort"
	"strings"

	statecontract "issueops/internal/contract/state"
)

type DoctorEntry struct {
	Name  string
	Path  string
	IsDir bool
}

type DoctorRecord struct {
	Key           string
	Path          string
	Record        statecontract.RecordEnvelope
	InvalidKey    string
	InvalidRecord bool
}

func InspectDoctor(dir string, entries []DoctorEntry, rows []DoctorRecord) statecontract.StateDoctorResult {
	result := statecontract.StateDoctorResult{
		OK: false, Healthy: false, StateDir: dir,
		ValidKeys: []string{}, Valid: []statecontract.StateListEntry{}, Issues: []statecontract.StateDoctorIssue{},
	}
	for _, entry := range entries {
		if issue, ok := inspectDoctorEntry(entry); ok {
			result.Issues = append(result.Issues, issue)
		}
	}
	for _, row := range rows {
		result.Checked++
		if row.InvalidKey != "" {
			result.Issues = append(result.Issues, statecontract.StateDoctorIssue{Path: row.Path, Key: row.Key, Severity: "error", Code: "invalid_key", Message: row.InvalidKey})
			continue
		}
		if row.InvalidRecord {
			result.Issues = append(result.Issues, statecontract.StateDoctorIssue{Path: row.Path, Key: row.Key, Severity: "error", Code: "invalid_state", Message: "invalid state"})
			continue
		}
		result.Valid = append(result.Valid, statecontract.StateListEntry{Key: row.Record.Key, UpdatedAt: row.Record.UpdatedAt, Bytes: row.Record.Bytes, SchemaVersion: row.Record.SchemaVersion})
		result.ValidKeys = append(result.ValidKeys, row.Record.Key)
	}
	sort.Strings(result.ValidKeys)
	sort.Slice(result.Valid, func(i, j int) bool { return result.Valid[i].Key < result.Valid[j].Key })
	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Path != result.Issues[j].Path {
			return result.Issues[i].Path < result.Issues[j].Path
		}
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Message < result.Issues[j].Message
	})
	result.OK = true
	result.Healthy = len(result.Issues) == 0
	return result
}

func inspectDoctorEntry(entry DoctorEntry) (statecontract.StateDoctorIssue, bool) {
	if entry.IsDir {
		if harnessOwnedStateDirectory(entry.Name) {
			return statecontract.StateDoctorIssue{}, false
		}
		return statecontract.StateDoctorIssue{Path: entry.Path, Severity: "warning", Code: "unexpected_directory", Message: "state directory contains an unexpected subdirectory"}, true
	}
	if harnessOwnedStateFile(entry.Name) {
		return statecontract.StateDoctorIssue{}, false
	}
	return statecontract.StateDoctorIssue{Path: entry.Path, Severity: "warning", Code: "unexpected_file", Message: "state directory contains an unexpected file"}, true
}

func harnessOwnedStateDirectory(name string) bool {
	switch name {
	case "projects", "worker", "loop", "issueops-benchmarks", "issueops_v1", "native-activation", "audit", "mcp-http":
		return true
	default:
		return false
	}
}

func harnessOwnedStateFile(name string) bool {
	for _, base := range []string{"issueops.db", "issueops.lock.db"} {
		if name == base || strings.HasPrefix(name, base+"-") {
			return true
		}
	}
	switch name {
	case statecontract.HookFailureLogFile, statecontract.RecordWriteLeaseFile, "hook-metrics.jsonl", ".last-store-maintain":
		return true
	default:
		return false
	}
}
