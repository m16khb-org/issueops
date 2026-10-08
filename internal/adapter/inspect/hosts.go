package inspect

import (
	"os"
	"path/filepath"
	"time"

	inspectcontract "issueops/internal/contract/inspect"
)

type hostSpec struct {
	name       string
	configPath string
	skillPath  string
	readConfig func(string) (hostEntry, error)
}

func (observer Observer) now() time.Time {
	if observer.Now != nil {
		return observer.Now()
	}
	return time.Now()
}

func (observer Observer) observeHosts(root, home, skillName string, options inspectcontract.Options) []inspectcontract.HostIntegration {
	codexHome := options.CodexHome
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	specs := []hostSpec{
		{inspectcontract.HostCodex, filepath.Join(codexHome, "config.toml"), filepath.Join(codexHome, "skills", skillName), readCodexEntry},
		{inspectcontract.HostClaude, filepath.Join(home, ".claude.json"), filepath.Join(home, ".claude", "skills", skillName), readJSONEntry},
		{inspectcontract.HostOmo, filepath.Join(home, ".omo", "mcp.json"), filepath.Join(home, ".omo", "agent", "skills", skillName), readJSONEntry},
		{inspectcontract.HostOmp, filepath.Join(home, ".omp", "agent", "mcp.json"), filepath.Join(home, ".omp", "agent", "skills", skillName), readJSONEntry},
	}
	observedAt := observer.now().Format(time.RFC3339)
	hosts := make([]inspectcontract.HostIntegration, 0, len(specs))
	entries := make([]hostEntry, 0, len(specs))
	for _, spec := range specs {
		host, entry := observeHostFiles(spec, root, skillName, observedAt)
		hosts = append(hosts, host)
		entries = append(entries, entry)
	}
	observer.applyReceipts(hosts, entries, options.HostReceipts, observedAt)
	return hosts
}

func observeHostFiles(spec hostSpec, root, skillName, observedAt string) (inspectcontract.HostIntegration, hostEntry) {
	host := inspectcontract.HostIntegration{Host: spec.name, ConfigPath: spec.configPath, SkillPath: spec.skillPath}
	host.Installed, host.Linked = observeSkill(spec.skillPath, filepath.Join(root, "skills", skillName), observedAt)
	entry, err := spec.readConfig(spec.configPath)
	if err != nil {
		host.Configured = observation(inspectcontract.ObservationFailed, spec.configPath, observedAt, reasonCode(err))
	} else {
		host.Transport = entry.transport
		host.Configured = observation(inspectcontract.ObservationVerified, spec.configPath, observedAt, "")
		host.Configured.ConfigSHA256 = entry.sha256
	}
	host.Discovered = notChecked()
	host.Connected = notChecked()
	host.Protocol = notChecked()
	return host, entry
}

func observeSkill(skillPath, sourcePath, observedAt string) (installed, linked inspectcontract.Observation) {
	info, err := os.Lstat(skillPath)
	if err != nil {
		missing := observation(inspectcontract.ObservationFailed, skillPath, observedAt, "skill_missing")
		return missing, missing
	}
	isLink := info.Mode()&os.ModeSymlink != 0
	resolved, resolveErr := filepath.EvalSymlinks(skillPath)
	if resolveErr != nil {
		broken := observation(inspectcontract.ObservationFailed, skillPath, observedAt, "broken_symlink")
		return broken, broken
	}
	if Exists(filepath.Join(skillPath, "SKILL.md")) {
		installed = observation(inspectcontract.ObservationVerified, skillPath, observedAt, "")
	} else {
		installed = observation(inspectcontract.ObservationFailed, skillPath, observedAt, "skill_md_missing")
	}
	switch {
	case !isLink:
		linked = observation(inspectcontract.ObservationFailed, skillPath, observedAt, "not_symlink")
	case sameResolvedPath(resolved, sourcePath):
		linked = observation(inspectcontract.ObservationVerified, skillPath, observedAt, "")
	default:
		linked = observation(inspectcontract.ObservationFailed, skillPath, observedAt, "link_target_mismatch")
	}
	return installed, linked
}

func sameResolvedPath(resolved, sourcePath string) bool {
	source, err := filepath.EvalSymlinks(sourcePath)
	return err == nil && source == resolved
}

func reasonCode(err error) string {
	if coded, ok := err.(configReadError); ok {
		return coded.code
	}
	return "config_unreadable"
}

func observation(status, source, observedAt, reason string) inspectcontract.Observation {
	return inspectcontract.Observation{Status: status, Source: source, ObservedAt: observedAt, Reason: reason, Features: []string{}}
}

func notChecked() inspectcontract.Observation {
	return inspectcontract.Observation{Status: inspectcontract.ObservationNotChecked, Reason: "host_receipt_required", Features: []string{}}
}
