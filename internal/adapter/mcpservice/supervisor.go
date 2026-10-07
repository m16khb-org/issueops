package mcpservice

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"issueops/internal/domain/policy"
)

// Runner executes one supervisor CLI command and returns its combined output.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

const loadOutputLimit = 2048

// runLoadStep runs one load command and, on failure, names the command and
// keeps the tail of its output: the exit status alone does not say why the
// supervisor refused the job.
func runLoadStep(ctx context.Context, run Runner, name string, args ...string) error {
	out, err := run(ctx, name, args...)
	if err == nil {
		return nil
	}
	command := strings.Join(append([]string{name}, args...), " ")
	output := policy.TailBytes(strings.TrimSpace(string(out)), loadOutputLimit)
	if output == "" {
		return fmt.Errorf("%s: %w", command, err)
	}
	return fmt.Errorf("%s: %w: %s", command, err, output)
}

type supervisor interface {
	command() string
	unitPath() string
	renderUnit(unit unitSpec) string
	load(ctx context.Context) error
	unload(ctx context.Context) error
	// supervised reports whether the job is loaded and the PID it supervises (0 when not running).
	supervised(ctx context.Context) (bool, int, error)
}

type unitSpec struct {
	Binary, Root, StateDir, LogPath string
}

func (spec unitSpec) environment() [][2]string {
	return [][2]string{{"ISSUEOPS_ROOT", spec.Root}, {"ISSUEOPS_STATE_DIR", spec.StateDir}}
}

type launchd struct {
	run   Runner
	label string
	uid   int
	home  string
}

func (l launchd) command() string { return "launchctl" }
func (l launchd) unitPath() string {
	return filepath.Join(l.home, "Library", "LaunchAgents", l.label+".plist")
}
func (l launchd) domain() string { return "gui/" + strconv.Itoa(l.uid) }
func (l launchd) target() string { return l.domain() + "/" + l.label }

func (l launchd) renderUnit(spec unitSpec) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>` + xmlText(l.label) + `</string>
	<key>ProgramArguments</key>
	<array>
		<string>` + xmlText(spec.Binary) + `</string>
		<string>mcp</string>
		<string>--http</string>
	</array>
	<key>EnvironmentVariables</key>
	<dict>
`)
	for _, pair := range spec.environment() {
		b.WriteString("\t\t<key>" + pair[0] + "</key>\n\t\t<string>" + xmlText(pair[1]) + "</string>\n")
	}
	b.WriteString(`	</dict>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<dict>
		<key>SuccessfulExit</key>
		<false/>
	</dict>
	<key>StandardOutPath</key>
	<string>` + xmlText(spec.LogPath) + `</string>
	<key>StandardErrorPath</key>
	<string>` + xmlText(spec.LogPath) + `</string>
</dict>
</plist>
`)
	return b.String()
}

func (l launchd) load(ctx context.Context) error {
	return runLoadStep(ctx, l.run, "launchctl", "bootstrap", l.domain(), l.unitPath())
}

func (l launchd) unload(ctx context.Context) error {
	_, err := l.run(ctx, "launchctl", "bootout", l.target())
	return err
}

func (l launchd) supervised(ctx context.Context) (bool, int, error) {
	out, err := l.run(ctx, "launchctl", "print", l.target())
	if err != nil {
		return false, 0, nil
	}
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok || strings.TrimSpace(key) != "pid" {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return true, 0, fmt.Errorf("parse launchd pid: %w", err)
		}
		return true, pid, nil
	}
	return true, 0, nil
}

type systemd struct {
	run  Runner
	unit string
	home string
}

func (s systemd) command() string { return "systemctl" }
func (s systemd) unitPath() string {
	return filepath.Join(s.home, ".config", "systemd", "user", s.unit)
}

func (s systemd) renderUnit(spec unitSpec) string {
	var b strings.Builder
	b.WriteString("[Unit]\nDescription=issueops shared MCP server\n\n[Service]\nType=simple\n")
	b.WriteString("ExecStart=" + systemdQuote(spec.Binary) + " mcp --http\n")
	for _, pair := range spec.environment() {
		b.WriteString("Environment=" + systemdQuote(pair[0]+"="+pair[1]) + "\n")
	}
	b.WriteString("Restart=on-failure\nStandardOutput=append:" + spec.LogPath + "\nStandardError=append:" + spec.LogPath + "\n\n[Install]\nWantedBy=default.target\n")
	return b.String()
}

func (s systemd) load(ctx context.Context) error {
	for _, args := range [][]string{{"--user", "daemon-reload"}, {"--user", "enable", s.unit}, {"--user", "start", s.unit}} {
		if err := runLoadStep(ctx, s.run, "systemctl", args...); err != nil {
			return err
		}
	}
	return nil
}

func (s systemd) unload(ctx context.Context) error {
	_, err := s.run(ctx, "systemctl", "--user", "stop", s.unit)
	return err
}

func (s systemd) supervised(ctx context.Context) (bool, int, error) {
	out, err := s.run(ctx, "systemctl", "--user", "show", "--property=ActiveState", "--property=MainPID", s.unit)
	if err != nil {
		return false, 0, nil
	}
	active, pid := false, 0
	for _, line := range strings.Split(string(out), "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			active = value == "active" || value == "activating" || value == "reloading"
		case "MainPID":
			parsed, err := strconv.Atoi(value)
			if err != nil {
				return active, 0, fmt.Errorf("parse systemd MainPID: %w", err)
			}
			pid = parsed
		}
	}
	return active, pid, nil
}

func xmlText(value string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(value))
	return b.String()
}

// systemdQuote quotes one word for ExecStart/Environment; systemd expands
// % specifiers and $ variables even inside quotes, so both are doubled.
func systemdQuote(value string) string {
	value = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "%", "%%", "$", "$$").Replace(value)
	return `"` + value + `"`
}
