package mcpservice

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestUnitsRunAbsoluteBinaryWithExplicitRootAndState(t *testing.T) {
	spec := unitSpec{Binary: "/opt/is sue&ops/bin/issueops", Root: "/opt/is sue&ops", StateDir: "/state/50%$HOME", LogPath: "/state/mcp-http/server.log"}
	plist := launchd{label: "io.issueops.mcp", uid: 501, home: "/Users/u"}.renderUnit(spec)
	for _, want := range []string{
		"<string>io.issueops.mcp</string>",
		"<string>/opt/is sue&amp;ops/bin/issueops</string>\n\t\t<string>mcp</string>\n\t\t<string>--http</string>",
		"<key>ISSUEOPS_ROOT</key>\n\t\t<string>/opt/is sue&amp;ops</string>",
		"<key>ISSUEOPS_STATE_DIR</key>\n\t\t<string>/state/50%$HOME</string>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
	if got := (launchd{label: "io.issueops.mcp", home: "/Users/u"}).unitPath(); got != "/Users/u/Library/LaunchAgents/io.issueops.mcp.plist" {
		t.Fatalf("launchd unit path = %s", got)
	}
	unit := systemd{unit: "issueops-mcp.service", home: "/home/u"}.renderUnit(spec)
	for _, want := range []string{
		`ExecStart="/opt/is sue&ops/bin/issueops" mcp --http`,
		`Environment="ISSUEOPS_ROOT=/opt/is sue&ops"`,
		`Environment="ISSUEOPS_STATE_DIR=/state/50%%$$HOME"`,
	} {
		if !strings.Contains(unit, want) {
			t.Fatalf("systemd unit missing %q:\n%s", want, unit)
		}
	}
	if got := (systemd{unit: "issueops-mcp.service", home: "/home/u"}).unitPath(); got != "/home/u/.config/systemd/user/issueops-mcp.service" {
		t.Fatalf("systemd unit path = %s", got)
	}
}

func TestSystemdSupervisedParsesActiveMainPID(t *testing.T) {
	var commands []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, name+" "+strings.Join(args, " "))
		return []byte("ActiveState=active\nMainPID=321\n"), nil
	}
	loaded, pid, err := systemd{run: run, unit: "issueops-mcp.service"}.supervised(t.Context())
	if err != nil || !loaded || pid != 321 {
		t.Fatalf("loaded=%v pid=%d err=%v", loaded, pid, err)
	}
	if commands[0] != "systemctl --user show --property=ActiveState --property=MainPID issueops-mcp.service" {
		t.Fatalf("commands = %v", commands)
	}
}

func TestLoadFailureNamesTheCommandAndItsOutput(t *testing.T) {
	exit := errors.New("exit status 1")
	for _, tc := range []struct {
		name    string
		sup     func(Runner) supervisor
		failing string
	}{
		{"systemd", func(run Runner) supervisor { return systemd{run: run, unit: "issueops-mcp.service", home: "/home/u"} }, "systemctl --user daemon-reload"},
		{"launchd", func(run Runner) supervisor {
			return launchd{run: run, label: "io.issueops.mcp", uid: 501, home: "/Users/u"}
		}, "launchctl bootstrap gui/501 /Users/u/Library/LaunchAgents/io.issueops.mcp.plist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, name string, args ...string) ([]byte, error) {
				return []byte("Failed to connect to bus: No medium found\n"), exit
			}
			err := tc.sup(run).load(t.Context())
			if !errors.Is(err, exit) {
				t.Fatalf("load error %v does not wrap the runner error", err)
			}
			for _, want := range []string{tc.failing, "Failed to connect to bus: No medium found"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("load error %q missing %q", err, want)
				}
			}
		})
	}
}

func TestLoadFailureBoundsLongOutputOnARuneBoundary(t *testing.T) {
	out := strings.Repeat("가", loadOutputLimit)
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		return []byte(out), errors.New("exit status 1")
	}
	err := systemd{run: run, unit: "issueops-mcp.service"}.load(t.Context())
	if err == nil {
		t.Fatal("load succeeded")
	}
	msg := err.Error()
	if !utf8.ValidString(msg) || len(msg) > loadOutputLimit+200 {
		t.Fatalf("load error is %d bytes or not valid UTF-8", len(msg))
	}
}
