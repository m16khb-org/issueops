package issueopscleanup

import (
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
			targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
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
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", nil)
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
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
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
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
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
	targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
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
		targets, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
		if !errors.Is(err, boom) || targets != nil {
			t.Fatalf("failAt=%d targets=%v err=%v", failAt, targets, err)
		}
	}
}

func samePath(a, b string) (bool, error) { return a == b, nil }

func cleanerFor(control WorkspaceProcessControl) WorkspaceCleaner {
	return WorkspaceCleaner{Processes: control, SamePath: samePath}
}

func TestWorkspaceCleanerObserveRejectsSourceCheckoutBeforeObserving(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy()}
	for name, same := range map[string]func(string, string) (bool, error){
		"is the repo":       func(string, string) (bool, error) { return true, nil },
		"path unresolvable": func(string, string) (bool, error) { return false, errors.New("stat") },
	} {
		t.Run(name, func(t *testing.T) {
			cleaner := cleanerFor(control)
			cleaner.SamePath = same
			_, missing := cleaner.Observe("/wt", "/repo")
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
		obs, missing := cleanerFor(&fakeProcessControl{observe: observe}).Observe("/wt", "/repo")
		if !slices.Equal(missing, []string{"workspace_processes_observable"}) || len(obs.Occupants) != 0 {
			t.Fatalf("%s: obs=%+v missing=%v", name, obs, missing)
		}
	}
}

func TestWorkspaceCleanerObserveReportsRequesterOccupancyAndReceipts(t *testing.T) {
	b := model.CleanupWorkspaceProcess{PID: 201, Command: "vim", StartedAt: "t0", Executable: "/usr/bin/vim"}
	control := &fakeProcessControl{observe: staticOccupancy(shellProcess, b, model.CleanupWorkspaceProcess{PID: testRequesterPID, Command: "issueops"})}
	obs, missing := cleanerFor(control).Observe("/wt", "/repo")
	if !slices.Equal(missing, []string{"requester_occupies_worktree"}) {
		t.Fatalf("missing=%v", missing)
	}
	if len(obs.Occupants) != 3 || len(obs.Receipts) != 3 || obs.Receipts[0].PID != testRequesterPID || obs.Receipts[2].PID != 201 {
		t.Fatalf("observation should carry pid-sorted receipts: %+v", obs.Receipts)
	}
}

func TestWorkspaceCleanerObserveAllowsOccupantsOutsideRequesterSession(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy(shellProcess)}
	obs, missing := cleanerFor(control).Observe("/wt", "/repo")
	if len(missing) != 0 || len(obs.Occupants) != 1 || len(obs.Receipts) != 1 {
		t.Fatalf("occupancy alone must not block the gate: obs=%+v missing=%v", obs, missing)
	}
}

func TestWorkspaceCleanerStopSignalsPreviewedProcesses(t *testing.T) {
	control := &fakeProcessControl{}
	control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
		if call == 1 {
			return occupancyOf(shellProcess), nil
		}
		return occupancyOf(), nil
	}
	stopped, err := cleanerFor(control).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess})
	if err != nil || len(stopped) != 1 || stopped[0].PID != shellProcess.PID {
		t.Fatalf("stopped=%v err=%v", stopped, err)
	}
	if !slices.Equal(control.signals, []string{"hup", "term"}) {
		t.Fatalf("signals=%v", control.signals)
	}
}

func TestWorkspaceCleanerStopRefusesRequesterOccupancy(t *testing.T) {
	control := &fakeProcessControl{observe: staticOccupancy(model.CleanupWorkspaceProcess{PID: testRequesterPID, Command: "issueops"})}
	_, err := cleanerFor(control).Stop("/wt", nil)
	if err == nil || !strings.Contains(err.Error(), "refuses to signal its own session") || len(control.signals) != 0 {
		t.Fatalf("err=%v signals=%v", err, control.signals)
	}
}

func TestWorkspaceProcessStopperObservesOnlyOncePerPhaseWhenGracefulStopSucceeds(t *testing.T) {
	control := &fakeProcessControl{}
	control.observe = func(call int) (port.CleanupWorkspaceOccupancy, error) {
		if call == 1 {
			return occupancyOf(shellProcess), nil
		}
		return occupancyOf(), nil
	}
	if _, err := (WorkspaceProcessStopper{Processes: control}).Stop("/wt", []model.CleanupWorkspaceProcess{shellProcess}); err != nil {
		t.Fatal(err)
	}
	// initial occupancy, grace-period release check, final confirmation
	if control.observed != 3 || slices.Contains(control.signals, "kill") {
		t.Fatalf("observed=%d signals=%v; no kill-phase observation when nothing survived", control.observed, control.signals)
	}
}
