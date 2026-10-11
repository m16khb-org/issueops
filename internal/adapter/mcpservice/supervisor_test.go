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

func TestLaunchdSupervisedDistinguishesNotLoadedIdleAndRunning(t *testing.T) {
	boom := errors.New("exit status 113")
	for _, tc := range []struct {
		name       string
		out        string
		err        error
		wantLoaded bool
		wantPID    int
		wantErr    bool
	}{
		{name: "job unknown", err: boom},
		{name: "loaded without a pid line", out: "state = waiting\n", wantLoaded: true},
		{name: "running", out: "gui/501/x = {\n\tstate = running\n\tpid = 88\n}\n", wantLoaded: true, wantPID: 88},
		{name: "unparsable pid", out: "pid = soon\n", wantLoaded: true, wantErr: true},
		{name: "other keys ending in pid are ignored", out: "last pid = 5\nstate = waiting\n", wantLoaded: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(context.Context, string, ...string) ([]byte, error) { return []byte(tc.out), tc.err }
			loaded, pid, err := launchd{run: run, label: "l", uid: 501}.supervised(t.Context())
			if loaded != tc.wantLoaded || pid != tc.wantPID || (err != nil) != tc.wantErr {
				t.Fatalf("loaded=%v pid=%d err=%v", loaded, pid, err)
			}
		})
	}
}

func TestSystemdSupervisedDistinguishesStatesAndBadMainPID(t *testing.T) {
	for _, tc := range []struct {
		name       string
		out        string
		err        error
		wantLoaded bool
		wantPID    int
		wantErr    bool
	}{
		{name: "unit unknown", err: errors.New("exit status 4")},
		{name: "inactive", out: "ActiveState=inactive\nMainPID=0\n"},
		{name: "activating counts as loaded", out: "ActiveState=activating\nMainPID=12\n", wantLoaded: true, wantPID: 12},
		{name: "reloading counts as loaded", out: "ActiveState=reloading\nMainPID=13\n", wantLoaded: true, wantPID: 13},
		{name: "failed is not loaded", out: "ActiveState=failed\nMainPID=0\n"},
		{name: "unparsable MainPID", out: "ActiveState=active\nMainPID=x\n", wantLoaded: true, wantErr: true},
		{name: "lines without a value are skipped", out: "garbage\nActiveState=active\nMainPID=4\n", wantLoaded: true, wantPID: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(context.Context, string, ...string) ([]byte, error) { return []byte(tc.out), tc.err }
			loaded, pid, err := systemd{run: run, unit: "u"}.supervised(t.Context())
			if loaded != tc.wantLoaded || pid != tc.wantPID || (err != nil) != tc.wantErr {
				t.Fatalf("loaded=%v pid=%d err=%v", loaded, pid, err)
			}
		})
	}
}

func TestUnloadTargetsTheJobAndPropagatesFailure(t *testing.T) {
	boom := errors.New("boom")
	var commands []string
	run := func(err error) Runner {
		return func(_ context.Context, name string, args ...string) ([]byte, error) {
			commands = append(commands, name+" "+strings.Join(args, " "))
			return nil, err
		}
	}
	if err := (launchd{run: run(nil), label: "io.x", uid: 7}).unload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := (systemd{run: run(nil), unit: "x.service"}).unload(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := []string{"launchctl bootout gui/7/io.x", "systemctl --user stop x.service"}
	if strings.Join(commands, "|") != strings.Join(want, "|") {
		t.Fatalf("commands = %v, want %v", commands, want)
	}
	if err := (launchd{run: run(boom)}).unload(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("launchd unload err = %v", err)
	}
	if err := (systemd{run: run(boom)}).unload(t.Context()); !errors.Is(err, boom) {
		t.Fatalf("systemd unload err = %v", err)
	}
}

func TestSystemdLoadStopsAtTheFirstFailingStep(t *testing.T) {
	var commands []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		commands = append(commands, strings.Join(args, " "))
		if args[1] == "enable" {
			return []byte("unit not found"), errors.New("exit status 1")
		}
		return nil, nil
	}
	err := systemd{run: run, unit: "x.service"}.load(t.Context())
	if err == nil || !strings.Contains(err.Error(), "systemctl --user enable x.service") {
		t.Fatalf("err = %v", err)
	}
	if strings.Join(commands, "|") != "--user daemon-reload|--user enable x.service" {
		t.Fatalf("commands = %v; start must not run after a failed enable", commands)
	}
}

func TestRunLoadStepOmitsEmptyOutputAndSucceedsQuietly(t *testing.T) {
	exit := errors.New("exit status 2")
	err := runLoadStep(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte(" \n"), exit }, "tool", "a", "b")
	if !errors.Is(err, exit) || err.Error() != "tool a b: exit status 2" {
		t.Fatalf("err = %v", err)
	}
	if err := runLoadStep(t.Context(), func(context.Context, string, ...string) ([]byte, error) { return []byte("noise"), nil }, "tool"); err != nil {
		t.Fatalf("err = %v", err)
	}
}
