package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

const testRequesterPID = 100

type fakeProcessControl struct {
	observe  func(call int) (port.CleanupWorkspaceOccupancy, error)
	observed int
	signals  []string
	waited   time.Duration
}

func (f *fakeProcessControl) Observe(string) (port.CleanupWorkspaceOccupancy, error) {
	f.observed++
	return f.observe(f.observed)
}
func (f *fakeProcessControl) RequesterPID() int    { return testRequesterPID }
func (f *fakeProcessControl) Hangup(pid int)       { f.signals = append(f.signals, "hup") }
func (f *fakeProcessControl) Terminate(pid int)    { f.signals = append(f.signals, "term") }
func (f *fakeProcessControl) Kill(pid int)         { f.signals = append(f.signals, "kill") }
func (f *fakeProcessControl) Wait(d time.Duration) { f.waited += d }

func occupancyOf(occupants ...model.CleanupWorkspaceProcess) port.CleanupWorkspaceOccupancy {
	return port.CleanupWorkspaceOccupancy{Occupants: occupants, Ancestry: map[int][]int{testRequesterPID: {1}}}
}

var shellProcess = model.CleanupWorkspaceProcess{PID: 200, Command: "zsh", StartedAt: "t1", Executable: "/bin/zsh"}

func staticOccupancy(occupants ...model.CleanupWorkspaceProcess) func(int) (port.CleanupWorkspaceOccupancy, error) {
	return func(int) (port.CleanupWorkspaceOccupancy, error) { return occupancyOf(occupants...), nil }
}

func TestWorkspaceProcessStopperRefusesWithoutSignalling(t *testing.T) {
	cases := []struct {
		name    string
		observe func(int) (port.CleanupWorkspaceOccupancy, error)
		wantErr error
		wantMsg string
	}{
		{name: "observation failure", observe: func(int) (port.CleanupWorkspaceOccupancy, error) {
			return port.CleanupWorkspaceOccupancy{}, errors.New("ps failed")
		}, wantMsg: "ps failed"},
		{name: "requester occupies worktree", observe: staticOccupancy(model.CleanupWorkspaceProcess{PID: testRequesterPID, Command: "issueops"}), wantMsg: "cleanup refuses to signal its own session"},
		{name: "occupant changed since preview", observe: staticOccupancy(model.CleanupWorkspaceProcess{PID: 200, Command: "zsh", StartedAt: "t2", Executable: "/bin/zsh"}), wantErr: domain.ErrCleanupOccupancyChanged},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			control := &fakeProcessControl{observe: tc.observe}
			targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil)
			if err == nil || targets != nil {
				t.Fatalf("Stop() = %v, %v; want refusal", targets, err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
			if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("err = %v, want containing %q", err, tc.wantMsg)
			}
			if len(control.signals) != 0 {
				t.Fatalf("signals sent despite refusal: %v", control.signals)
			}
		})
	}
}

func TestWorkspaceProcessStopperNoOccupantsSignalsNothing(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy()}
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", nil, nil)
	if err != nil || len(targets) != 0 || len(control.signals) != 0 || control.observed != 1 {
		t.Fatalf("targets=%v err=%v signals=%v observed=%d", targets, err, control.signals, control.observed)
	}
}

func TestWorkspaceProcessStopperGracefulStopSkipsKill(t *testing.T) {
	control := &fakeProcessControl{}
	control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
		if call == 1 {
			return occupancyOf(shellProcess), nil
		}
		return occupancyOf(), nil
	}
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil)
	if err != nil || len(targets) != 1 || targets[0].PID != 200 {
		t.Fatalf("targets=%v err=%v", targets, err)
	}
	if !slices.Equal(control.signals, []string{"hup", "term"}) || control.waited != 0 {
		t.Fatalf("signals=%v waited=%v; want hup,term with no waiting", control.signals, control.waited)
	}
}

func TestWorkspaceProcessStopperEscalatesToKillAfterGracePeriod(t *testing.T) {
	control := &fakeProcessControl{}
	control.observe = func(int) (port.CleanupWorkspaceOccupancy, error) {
		if slices.Contains(control.signals, "kill") {
			return occupancyOf(), nil
		}
		return occupancyOf(shellProcess), nil
	}
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil)
	if err != nil || len(targets) != 1 {
		t.Fatalf("targets=%v err=%v", targets, err)
	}
	if !slices.Equal(control.signals, []string{"hup", "term", "kill"}) {
		t.Fatalf("signals=%v", control.signals)
	}
	if control.waited != cleanupStopGracePeriod {
		t.Fatalf("waited %v before kill, want the %v grace period", control.waited, cleanupStopGracePeriod)
	}
}

func TestWorkspaceProcessStopperFailsWhenProcessSurvivesKill(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy(shellProcess)}
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil)
	if err == nil || targets != nil || !strings.Contains(err.Error(), "still occupy /wt after HUP/TERM/KILL: 200:zsh") {
		t.Fatalf("targets=%v err=%v", targets, err)
	}
	if control.waited != cleanupStopGracePeriod+cleanupStopKillPeriod {
		t.Fatalf("waited %v, want grace+kill periods", control.waited)
	}
}

func TestWorkspaceProcessStopperReportsObservationFailuresAfterSignalling(t *testing.T) {
	boom := errors.New("observe boom")
	for _, failAt := range []int{2, 3} {
		control := &fakeProcessControl{}
		control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
			switch {
			case call == failAt:
				return port.CleanupWorkspaceOccupancy{}, boom
			case call == 1:
				return occupancyOf(shellProcess), nil
			}
			return occupancyOf(), nil
		}
		targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil)
		if !errors.Is(err, boom) || targets != nil {
			t.Fatalf("failAt=%d targets=%v err=%v", failAt, targets, err)
		}
	}
}

func TestWorkspaceProcessStopperExcludesProtectedPIDs(t *testing.T) {
	orcaApp := model.CleanupWorkspaceProcess{PID: 300, Command: "Orca"}
	control := &fakeProcessControl{observe: staticOccupancy(orcaApp)}
	// The protected pid is in the preview, so only the exclusion keeps it from being a valid target.
	_, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{orcaApp}, map[int]bool{300: true})
	if err == nil || !strings.Contains(err.Error(), "refuses to signal its own session") || errors.Is(err, domain.ErrCleanupOccupancyChanged) || len(control.signals) != 0 {
		t.Fatalf("err=%v signals=%v; excluded pid must never be signalled", err, control.signals)
	}
}

type fakeOrca struct {
	status     port.OrcaStatus
	statusErr  error
	all        []port.OrcaTerminal
	allErr     error
	byPath     func(call int) ([]port.OrcaTerminal, error)
	byPathCall int
	closeErr   map[string]error
	closed     []string
}

func (f *fakeOrca) Status(context.Context) (port.OrcaStatus, error) { return f.status, f.statusErr }
func (f *fakeOrca) ListAllTerminals(context.Context) ([]port.OrcaTerminal, error) {
	return f.all, f.allErr
}
func (f *fakeOrca) ListWorktreeTerminalsByPath(context.Context, string) ([]port.OrcaTerminal, error) {
	f.byPathCall++
	if f.byPath == nil {
		return nil, nil
	}
	return f.byPath(f.byPathCall)
}
func (f *fakeOrca) CloseTerminal(_ context.Context, handle string) error {
	f.closed = append(f.closed, handle)
	return f.closeErr[handle]
}

func readyOrca() *fakeOrca {
	return &fakeOrca{status: port.OrcaStatus{RuntimeReachable: true, RuntimeState: "ready", GraphState: "ready", AppPID: 900}}
}

func samePath(a, b string) (bool, error) { return a == b, nil }

func cleanerFor(control WorkspaceProcessControl, orca port.CleanupOrcaTerminals) WorkspaceCleaner {
	return WorkspaceCleaner{Processes: control, Orca: orca, SamePath: samePath}
}

func boundRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{Repo: "/repo", Execution: &model.Execution{Orca: &model.OrcaBinding{WorktreeID: "wt-1"}}}
}

func term(handle, path string) port.OrcaTerminal {
	return port.OrcaTerminal{Handle: handle, WorktreePath: path}
}

func TestWorkspaceCleanerObserveRejectsSourceCheckoutBeforeObserving(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy()}
	for name, same := range map[string]func(string, string) (bool, error){
		"is the repo":       func(string, string) (bool, error) { return true, nil },
		"path unresolvable": func(string, string) (bool, error) { return false, errors.New("stat") },
	} {
		t.Run(name, func(t *testing.T) {
			cleaner := cleanerFor(control, nil)
			cleaner.SamePath = same
			_, missing := cleaner.Observe(t.Context(), boundRecord(), "/wt")
			if !slices.Equal(missing, []string{"worktree_is_source_checkout"}) || control.observed != 0 {
				t.Fatalf("missing=%v observed=%d", missing, control.observed)
			}
		})
	}
}

func TestWorkspaceCleanerObserveFailsClosedOnUnobservableProcesses(t *testing.T) {
	failing := func(int) (port.CleanupWorkspaceOccupancy, error) {
		return port.CleanupWorkspaceOccupancy{}, errors.New("ps")
	}
	noAncestry := func(int) (port.CleanupWorkspaceOccupancy, error) {
		return port.CleanupWorkspaceOccupancy{Occupants: []model.CleanupWorkspaceProcess{shellProcess}}, nil
	}
	for name, observe := range map[string]func(int) (port.CleanupWorkspaceOccupancy, error){"observe error": failing, "requester ancestry missing": noAncestry} {
		obs, missing := cleanerFor(&fakeProcessControl{observe: observe}, nil).Observe(t.Context(), boundRecord(), "/wt")
		if !slices.Equal(missing, []string{"workspace_processes_observable"}) || len(obs.Occupants) != 0 {
			t.Fatalf("%s: obs=%+v missing=%v", name, obs, missing)
		}
	}
}

func TestWorkspaceCleanerObserveReportsRequesterOccupancyAndReceipts(t *testing.T) {
	b := model.CleanupWorkspaceProcess{PID: 201, Command: "vim", StartedAt: "t0", Executable: "/usr/bin/vim"}
	control := &fakeProcessControl{observe: staticOccupancy(shellProcess, b, model.CleanupWorkspaceProcess{PID: testRequesterPID, Command: "issueops"})}
	obs, missing := cleanerFor(control, nil).Observe(t.Context(), model.IssueOpsRecord{Repo: "/repo"}, "/wt")
	if !slices.Equal(missing, []string{"requester_occupies_worktree"}) {
		t.Fatalf("missing=%v", missing)
	}
	if len(obs.Occupants) != 3 || len(obs.Receipts) != 3 || obs.Receipts[0].PID != testRequesterPID || obs.Receipts[2].PID != 201 {
		t.Fatalf("observation should carry pid-sorted receipts: %+v", obs.Receipts)
	}
}

func TestWorkspaceCleanerObserveWithoutOrcaSurface(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy(shellProcess)}
	obs, missing := cleanerFor(control, nil).Observe(t.Context(), boundRecord(), "/wt")
	if !slices.Equal(missing, []string{"orca_runtime_ready"}) || len(obs.Occupants) != 1 {
		t.Fatalf("bound: obs=%+v missing=%v", obs, missing)
	}
	_, missing = cleanerFor(control, nil).Observe(t.Context(), model.IssueOpsRecord{Repo: "/repo"}, "/wt")
	if len(missing) != 0 {
		t.Fatalf("unbound cycle without Orca must not require the runtime: %v", missing)
	}
}

func TestWorkspaceCleanerObserveListsSortedTerminalsWhenRuntimeReady(t *testing.T) {
	orca := readyOrca()
	orca.byPath = func(int) ([]port.OrcaTerminal, error) {
		return []port.OrcaTerminal{term(" term-b ", "/wt"), term("term-a", " /wt ")}, nil
	}
	cleaner := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca)
	cleaner.SamePath = func(a, b string) (bool, error) { return a == b || (a == " /wt " && b == "/wt"), nil }
	for _, record := range []model.IssueOpsRecord{boundRecord(), {Repo: "/repo"}} {
		obs, missing := cleaner.Observe(t.Context(), record, "/wt")
		if len(missing) != 0 || !obs.RuntimeReady || obs.AppPID != 900 || !slices.Equal(obs.Terminals, []string{"term-a", "term-b"}) {
			t.Fatalf("obs=%+v missing=%v", obs, missing)
		}
	}
}

func TestWorkspaceCleanerObserveRuntimeReadinessDependsOnBinding(t *testing.T) {
	cases := []struct {
		name        string
		status      port.OrcaStatus
		statusErr   error
		record      model.IssueOpsRecord
		wantMissing []string
		wantReady   bool
	}{
		{name: "bound runtime unreachable", status: port.OrcaStatus{RuntimeState: "ready", GraphState: "ready"}, record: boundRecord(), wantMissing: []string{"orca_runtime_ready"}},
		{name: "bound graph not ready", status: port.OrcaStatus{RuntimeReachable: true, RuntimeState: "ready", GraphState: "syncing"}, record: boundRecord(), wantMissing: []string{"orca_runtime_ready"}},
		{name: "bound status error", statusErr: errors.New("orca down"), record: boundRecord(), wantMissing: []string{"orca_runtime_ready"}},
		{name: "unbound runtime down is tolerated", status: port.OrcaStatus{RuntimeState: "starting"}, record: model.IssueOpsRecord{Repo: "/repo"}},
		{name: "unbound ignores graph state", status: port.OrcaStatus{RuntimeReachable: true, RuntimeState: "ready", GraphState: "syncing"}, record: model.IssueOpsRecord{Repo: "/repo"}, wantReady: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orca := &fakeOrca{status: tc.status, statusErr: tc.statusErr}
			obs, missing := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca).Observe(t.Context(), tc.record, "/wt")
			if !slices.Equal(missing, tc.wantMissing) && !(len(missing) == 0 && len(tc.wantMissing) == 0) {
				t.Fatalf("missing=%v want %v", missing, tc.wantMissing)
			}
			if obs.RuntimeReady != tc.wantReady {
				t.Fatalf("RuntimeReady=%v want %v", obs.RuntimeReady, tc.wantReady)
			}
		})
	}
}

func TestWorkspaceCleanerObserveRejectsUnreadableOrMalformedTerminalInventory(t *testing.T) {
	cases := map[string]func(int) ([]port.OrcaTerminal, error){
		"list error":   func(int) ([]port.OrcaTerminal, error) { return nil, errors.New("list") },
		"empty handle": func(int) ([]port.OrcaTerminal, error) { return []port.OrcaTerminal{term("", "/wt")}, nil },
		"duplicate handle": func(int) ([]port.OrcaTerminal, error) {
			return []port.OrcaTerminal{term("a", "/wt"), term("a", "/wt")}, nil
		},
		"foreign worktree": func(int) ([]port.OrcaTerminal, error) { return []port.OrcaTerminal{term("a", "/other")}, nil },
	}
	for name, byPath := range cases {
		t.Run(name, func(t *testing.T) {
			orca := readyOrca()
			orca.byPath = byPath
			obs, missing := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca).Observe(t.Context(), boundRecord(), "/wt")
			if !slices.Equal(missing, []string{"orca_terminals_observable"}) || len(obs.Terminals) != 0 {
				t.Fatalf("obs=%+v missing=%v", obs, missing)
			}
		})
	}
}

func TestWorkspaceCleanerObserveResolvesRequesterTerminal(t *testing.T) {
	sameErr := errors.New("stat")
	cases := []struct {
		name         string
		pane, handle string
		all          []port.OrcaTerminal
		allErr       error
		notReady     bool
		samePathErr  bool
		want         []string
	}{
		{name: "runtime not ready", handle: "h", notReady: true, want: []string{"requester_terminal_unresolved"}},
		{name: "inventory error", handle: "h", allErr: errors.New("x"), want: []string{"requester_terminal_unresolved"}},
		{name: "not in inventory", handle: "h", all: []port.OrcaTerminal{term("other", "/elsewhere")}, want: []string{"requester_terminal_unresolved"}},
		{name: "no worktree path", handle: "h", all: []port.OrcaTerminal{term("h", "  ")}, want: []string{"requester_terminal_unresolved"}},
		{name: "ambiguous handle", handle: "h", all: []port.OrcaTerminal{term("h", "/elsewhere"), term("h", "/elsewhere")}, want: []string{"requester_terminal_unresolved"}},
		{name: "path comparison fails", handle: "h", all: []port.OrcaTerminal{term("h", "/elsewhere")}, samePathErr: true, want: []string{"requester_terminal_unresolved"}},
		{name: "requester terminal inside worktree by handle", handle: "h", all: []port.OrcaTerminal{term("h", "/wt")}, want: []string{"requester_terminal_outside_worktree"}},
		{name: "requester terminal inside worktree by pane key", pane: "tab:leaf", all: []port.OrcaTerminal{{Handle: "z", TabID: "tab", LeafID: "leaf", WorktreePath: "/wt"}}, want: []string{"requester_terminal_outside_worktree"}},
		{name: "requester terminal elsewhere", handle: "h", all: []port.OrcaTerminal{term("h", "/elsewhere")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			orca := readyOrca()
			if tc.notReady {
				orca.status.RuntimeState = "starting"
			}
			orca.all, orca.allErr = tc.all, tc.allErr
			cleaner := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca)
			cleaner.PaneKey, cleaner.Handle = tc.pane, tc.handle
			if tc.samePathErr {
				cleaner.SamePath = func(a, b string) (bool, error) {
					if a == "/elsewhere" {
						return false, sameErr
					}
					return a == b, nil
				}
			}
			_, missing := cleaner.Observe(t.Context(), model.IssueOpsRecord{Repo: "/repo"}, "/wt")
			if !slices.Equal(missing, tc.want) && !(len(missing) == 0 && len(tc.want) == 0) {
				t.Fatalf("missing=%v want %v", missing, tc.want)
			}
		})
	}
}

func TestWorkspaceCleanerStopClosesTerminalsThenSignalsProcesses(t *testing.T) {
	orca := readyOrca()
	control := &fakeProcessControl{}
	control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
		if call == 1 {
			return occupancyOf(shellProcess), nil
		}
		return occupancyOf(), nil
	}
	stopped, closed, err := cleanerFor(control, orca).Stop(t.Context(), "/wt", []model.CleanupWorkspaceProcess{shellProcess}, []string{"term-a", "term-b"}, true, 0)
	if err != nil || closed != 2 || len(stopped) != 1 {
		t.Fatalf("stopped=%v closed=%d err=%v", stopped, closed, err)
	}
	if !slices.Equal(orca.closed, []string{"term-a", "term-b"}) || orca.byPathCall != 2 {
		t.Fatalf("closed=%v inventoryChecks=%d; want exact handles and a recheck before and after signalling", orca.closed, orca.byPathCall)
	}
}

func TestWorkspaceCleanerStopProtectsOrcaAppPID(t *testing.T) {
	orcaApp := model.CleanupWorkspaceProcess{PID: 900, Command: "Orca"}
	control := &fakeProcessControl{observe: staticOccupancy(orcaApp)}
	_, _, err := cleanerFor(control, readyOrca()).Stop(t.Context(), "/wt", []model.CleanupWorkspaceProcess{orcaApp}, nil, false, 900)
	if err == nil || !strings.Contains(err.Error(), "refuses to signal its own session") || errors.Is(err, domain.ErrCleanupOccupancyChanged) || len(control.signals) != 0 {
		t.Fatalf("err=%v signals=%v; the Orca app pid must never be signalled", err, control.signals)
	}
}

func TestWorkspaceCleanerStopFailsClosedOnTerminalProblems(t *testing.T) {
	t.Run("terminals previewed without orca surface", func(t *testing.T) {
		control := &fakeProcessControl{observe: staticOccupancy()}
		_, closed, err := cleanerFor(control, nil).Stop(t.Context(), "/wt", nil, []string{"a"}, false, 0)
		if err == nil || closed != 0 || control.observed != 0 {
			t.Fatalf("err=%v closed=%d observed=%d", err, closed, control.observed)
		}
	})
	t.Run("close failure stops before the next handle and signals", func(t *testing.T) {
		orca := readyOrca()
		orca.closeErr = map[string]error{"b": errors.New("pty alive")}
		control := &fakeProcessControl{observe: staticOccupancy(shellProcess)}
		_, closed, err := cleanerFor(control, orca).Stop(t.Context(), "/wt", []model.CleanupWorkspaceProcess{shellProcess}, []string{"a", "b", "c"}, true, 0)
		if err == nil || !strings.Contains(err.Error(), "close Orca terminal b") || closed != 1 {
			t.Fatalf("err=%v closed=%d", err, closed)
		}
		if !slices.Equal(orca.closed, []string{"a", "b"}) || len(control.signals) != 0 {
			t.Fatalf("closed=%v signals=%v", orca.closed, control.signals)
		}
	})
	t.Run("terminals remain after close", func(t *testing.T) {
		orca := readyOrca()
		orca.byPath = func(int) ([]port.OrcaTerminal, error) { return []port.OrcaTerminal{term("a", "/wt")}, nil }
		control := &fakeProcessControl{observe: staticOccupancy()}
		_, closed, err := cleanerFor(control, orca).Stop(t.Context(), "/wt", nil, []string{"a"}, true, 0)
		if err == nil || !strings.Contains(err.Error(), "Orca terminals remain before deletion") || closed != 1 || control.observed != 0 {
			t.Fatalf("err=%v closed=%d observed=%d", err, closed, control.observed)
		}
	})
	t.Run("inventory unreadable after close", func(t *testing.T) {
		orca := readyOrca()
		orca.byPath = func(int) ([]port.OrcaTerminal, error) { return nil, errors.New("list") }
		_, _, err := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca).Stop(t.Context(), "/wt", nil, []string{"a"}, true, 0)
		if err == nil || !strings.Contains(err.Error(), "observe Orca terminals before deletion") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("malformed inventory after close", func(t *testing.T) {
		orca := readyOrca()
		orca.byPath = func(int) ([]port.OrcaTerminal, error) { return []port.OrcaTerminal{term("", "/wt")}, nil }
		_, _, err := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca).Stop(t.Context(), "/wt", nil, []string{"a"}, true, 0)
		if err == nil || !strings.Contains(err.Error(), "malformed Orca terminal inventory") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("runtime previewed ready without orca surface", func(t *testing.T) {
		stopped, _, err := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, nil).Stop(t.Context(), "/wt", nil, nil, true, 0)
		if err == nil || len(stopped) != 0 {
			t.Fatalf("stopped=%v err=%v", stopped, err)
		}
	})
	t.Run("terminal reappears after signalling", func(t *testing.T) {
		orca := readyOrca()
		orca.byPath = func(int) ([]port.OrcaTerminal, error) { return []port.OrcaTerminal{term("late", "/wt")}, nil }
		_, _, err := cleanerFor(&fakeProcessControl{observe: staticOccupancy()}, orca).Stop(t.Context(), "/wt", nil, nil, true, 0)
		if err == nil || !strings.Contains(err.Error(), "Orca terminals remain") {
			t.Fatalf("err=%v", err)
		}
	})
	t.Run("process stop failure surfaces after terminals closed", func(t *testing.T) {
		orca := readyOrca()
		control := &fakeProcessControl{observe: staticOccupancy(model.CleanupWorkspaceProcess{PID: testRequesterPID})}
		_, closed, err := cleanerFor(control, orca).Stop(t.Context(), "/wt", nil, []string{"a"}, true, 0)
		if err == nil || closed != 1 {
			t.Fatalf("err=%v closed=%d", err, closed)
		}
	})
}

func TestWorkspaceCleanerStopWithoutTerminalsNeedsNoOrcaSurface(t *testing.T) {
	t.Run("no orca surface", func(t *testing.T) {
		control := &fakeProcessControl{observe: staticOccupancy()}
		_, closed, err := cleanerFor(control, nil).Stop(t.Context(), "/wt", nil, nil, false, 0)
		if err != nil || closed != 0 {
			t.Fatalf("closed=%d err=%v", closed, err)
		}
	})
	t.Run("orca not consulted", func(t *testing.T) {
		orca := readyOrca()
		control := &fakeProcessControl{observe: staticOccupancy()}
		if _, _, err := cleanerFor(control, orca).Stop(t.Context(), "/wt", nil, nil, false, 0); err != nil {
			t.Fatal(err)
		}
		if len(orca.closed) != 0 || orca.byPathCall != 0 {
			t.Fatalf("closed=%v inventoryChecks=%d; nothing was previewed", orca.closed, orca.byPathCall)
		}
	})
}

func TestWorkspaceProcessStopperObservesOnlyOncePerPhaseWhenGracefulStopSucceeds(t *testing.T) {
	control := &fakeProcessControl{}
	control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
		if call == 1 {
			return occupancyOf(shellProcess), nil
		}
		return occupancyOf(), nil
	}
	if _, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}, nil); err != nil {
		t.Fatal(err)
	}
	// initial occupancy, grace-period release check, final confirmation
	if control.observed != 3 || slices.Contains(control.signals, "kill") {
		t.Fatalf("observed=%d signals=%v; no kill-phase observation when nothing survived", control.observed, control.signals)
	}
}
