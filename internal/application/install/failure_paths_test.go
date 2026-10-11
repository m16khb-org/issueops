package install

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	activationcontract "issueops/internal/contract/nativeactivation"
	"issueops/internal/port"
)

// scripted drives every collaborator of RunTransaction with injectable failures
// and records the call order.
type scripted struct {
	events []string

	preparePathErr error
	hostPlan       *port.NativeInstallResult // result of the dry-run (preflight) Install call
	hostPlanErr    error
	planShellErr   error // returned by every PlanShell call
	planShellOnce  bool  // fail only the first PlanShell call
	planShellCalls int
	prepareHostErr error
	applyErr       error
	installOK      bool // result.OK of the real Install call
	installErr     error
	sealResult     activationcontract.Result
	sealErr        error
	beginErr       error
	abortErr       error
	finalizeErr    error
	rollbackHost   error
	rollbackPath   error
	commandPath    *port.ManagedCommandPathResult
}

func (s *scripted) ev(name string) { s.events = append(s.events, name) }

func (s *scripted) PreparePath(preflight *port.NativeInstallResult, _ port.NativeInstallRequest, _, _ string) (PathTransaction, error) {
	s.ev("prepare-path")
	if s.preparePathErr != nil {
		return nil, s.preparePathErr
	}
	preflight.Links = append(preflight.Links, port.InstallLink{Path: "preflight-link"})
	preflight.CommandPath = s.commandPath
	return s, nil
}

func (s *scripted) Install(request port.NativeInstallRequest) (port.NativeInstallResult, error) {
	if request.DryRun {
		s.ev("plan-hosts")
		if s.hostPlan != nil || s.hostPlanErr != nil {
			plan := port.NativeInstallResult{}
			if s.hostPlan != nil {
				plan = *s.hostPlan
			}
			return plan, s.hostPlanErr
		}
		return port.NativeInstallResult{OK: true, Links: []port.InstallLink{{Path: "plan-link"}}}, nil
	}
	s.ev("install-hosts")
	return port.NativeInstallResult{OK: s.installOK, Links: []port.InstallLink{{Path: "host-link"}}}, s.installErr
}

func (s *scripted) PlanShell(*port.NativeInstallResult, port.NativeInstallRequest, string) error {
	s.ev("plan-shell")
	s.planShellCalls++
	if s.planShellErr != nil && (!s.planShellOnce || s.planShellCalls == 2) {
		return s.planShellErr
	}
	return nil
}

func (s *scripted) PrepareHost(port.NativeInstallResult) (HostTransaction, error) {
	s.ev("snapshot-hosts")
	if s.prepareHostErr != nil {
		return nil, s.prepareHostErr
	}
	return s, nil
}

func (s *scripted) AppendUpstream(*port.NativeInstallResult, string, bool) { s.ev("upstream") }
func (s *scripted) RemoveRetiredState(*port.NativeInstallResult, bool)     { s.ev("retired-state") }

func (s *scripted) Apply(*port.NativeInstallResult) error { s.ev("apply-path"); return s.applyErr }
func (s *scripted) Rollback(*port.NativeInstallResult) error {
	s.ev("rollback-path")
	return s.rollbackPath
}
func (s *scripted) Finalize(*port.NativeInstallResult) error {
	s.ev("finalize-path")
	return s.finalizeErr
}
func (s *scripted) RollbackHosts() error { s.ev("rollback-hosts"); return s.rollbackHost }

func (s *scripted) Begin(context.Context, activationcontract.Request) (activationcontract.Result, error) {
	s.ev("begin")
	if s.beginErr != nil {
		return activationcontract.Result{}, s.beginErr
	}
	return activationcontract.Result{TransitionID: "new-transition"}, nil
}

func (s *scripted) Seal(_ context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	s.ev("seal:" + request.TransitionID)
	return s.sealResult, s.sealErr
}

func (s *scripted) Abort(_ context.Context, request activationcontract.Request) (activationcontract.Result, error) {
	s.ev("abort:" + request.TransitionID)
	return activationcontract.Result{TransitionID: request.TransitionID}, s.abortErr
}

func sealedOK() activationcontract.Result {
	return activationcontract.Result{OK: true, Sealed: true, Receipt: &activationcontract.Receipt{}}
}

func TestRunTransactionAbortStepOnlyAbortsWithRequestedTransition(t *testing.T) {
	s := &scripted{}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Step: "abort", TransitionID: "t-9"}, s, s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.events, []string{"abort:t-9"}) {
		t.Fatalf("abort step must touch nothing but Abort: %v", s.events)
	}
	if outcome.Install != nil || outcome.Activation == nil || outcome.Activation.TransitionID != "t-9" {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestRunTransactionAbortStepWrapsAbortFailure(t *testing.T) {
	cause := errors.New("ledger locked")
	s := &scripted{abortErr: cause}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Step: "abort", TransitionID: "t-9"}, s, s)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "abort native activation") {
		t.Fatalf("err = %v", err)
	}
	if outcome.Activation != nil || outcome.Install != nil {
		t.Fatalf("outcome = %+v", outcome)
	}
}

func TestRunTransactionPreparePathFailureStopsBeforeAnyHostWork(t *testing.T) {
	cause := errors.New("path conflict")
	s := &scripted{preparePathErr: cause}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if !errors.Is(err, cause) || outcome.Install != nil {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if !reflect.DeepEqual(s.events, []string{"prepare-path"}) {
		t.Fatalf("events = %v", s.events)
	}
}

func TestRunTransactionDryRunMergesPreflightAheadOfHostPlan(t *testing.T) {
	s := &scripted{commandPath: &port.ManagedCommandPathResult{}}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Install: port.NativeInstallRequest{DryRun: true}}, s, s)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, link := range outcome.Install.Links {
		got = append(got, link.Path)
	}
	if !reflect.DeepEqual(got, []string{"preflight-link", "plan-link"}) {
		t.Fatalf("links = %v", got)
	}
	if outcome.Install.CommandPath != s.commandPath {
		t.Fatalf("command path not carried from preflight: %+v", outcome.Install.CommandPath)
	}
}

func TestRunTransactionDryRunPropagatesInstallError(t *testing.T) {
	cause := errors.New("host plan broke")
	s := &scripted{hostPlanErr: cause}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Install: port.NativeInstallRequest{DryRun: true}}, s, s)
	if !errors.Is(err, cause) || outcome.Install == nil {
		t.Fatalf("outcome=%+v err=%v", outcome, err)
	}
	if !reflect.DeepEqual(s.events, []string{"prepare-path", "plan-hosts", "upstream", "retired-state"}) {
		t.Fatalf("dry run still reports planned upstream/retired state: %v", s.events)
	}
}

func TestRunTransactionHostPreflightFailuresNeverWriteOrBegin(t *testing.T) {
	for _, tc := range []struct {
		name      string
		script    *scripted
		wantErr   string
		wantCause error
		events    []string
	}{
		{
			name: "dry-run error", script: &scripted{hostPlanErr: errors.New("plan boom")}, wantErr: "plan boom",
			events: []string{"prepare-path", "plan-hosts"},
		},
		{
			name: "dry-run reports ok=false", script: &scripted{hostPlan: &port.NativeInstallResult{OK: false}},
			wantErr: "dry-run preflight reported ok=false", events: []string{"prepare-path", "plan-hosts"},
		},
		{
			name: "shell plan error", script: &scripted{planShellErr: errors.New("shell boom")}, wantErr: "shell boom",
			events: []string{"prepare-path", "plan-hosts", "plan-shell"},
		},
		{
			name: "host snapshot error", script: &scripted{prepareHostErr: errors.New("snapshot boom")}, wantErr: "snapshot boom",
			events: []string{"prepare-path", "plan-hosts", "plan-shell", "snapshot-hosts"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := RunTransaction(t.Context(), TransactionRequest{}, tc.script, tc.script)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if outcome.Install == nil || outcome.Install.OK || outcome.Install.AbortRequired || outcome.Install.Committed {
				t.Fatalf("outcome = %+v", outcome.Install)
			}
			var links []string
			for _, link := range outcome.Install.Links {
				links = append(links, link.Path)
			}
			if links[0] != "preflight-link" {
				t.Fatalf("preflight links must lead: %v", links)
			}
			if !reflect.DeepEqual(tc.script.events, tc.events) {
				t.Fatalf("events = %v, want %v", tc.script.events, tc.events)
			}
		})
	}
}

func TestRunTransactionSealStepPreflightFailureRequiresAbort(t *testing.T) {
	s := &scripted{hostPlanErr: errors.New("plan boom"), commandPath: &port.ManagedCommandPathResult{}}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Step: "seal", TransitionID: "t-4"}, s, s)
	if err == nil {
		t.Fatal("want error")
	}
	got := outcome.Install
	if got.TransitionID != "t-4" || !got.AbortRequired || !got.CommandPath.AbortRequired {
		t.Fatalf("seal-step failure must hand abort duty to caller: %+v", got)
	}
	for _, event := range s.events {
		if strings.HasPrefix(event, "abort") {
			t.Fatalf("seal step must not abort by itself: %v", s.events)
		}
	}
}

func TestRunTransactionBeginFailureAppliesNothing(t *testing.T) {
	cause := errors.New("ledger down")
	s := &scripted{beginErr: cause}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), "begin native activation") {
		t.Fatalf("err = %v", err)
	}
	if outcome.Install != nil || outcome.Activation != nil {
		t.Fatalf("outcome = %+v", outcome)
	}
	if !reflect.DeepEqual(s.events, []string{"prepare-path", "plan-hosts", "plan-shell", "snapshot-hosts", "begin"}) {
		t.Fatalf("events = %v", s.events)
	}
}

func TestRunTransactionSealStepReusesCallerTransitionWithoutBegin(t *testing.T) {
	s := &scripted{installOK: true, sealResult: sealedOK()}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Step: "seal", TransitionID: "t-7"}, s, s)
	if err != nil {
		t.Fatal(err)
	}
	if !outcome.Install.Committed || outcome.Install.TransitionID != "t-7" {
		t.Fatalf("outcome = %+v", outcome.Install)
	}
	if !contains(s.events, "seal:t-7") || contains(s.events, "begin") {
		t.Fatalf("events = %v", s.events)
	}
}

func TestRunTransactionFullFlowSealsTheTransitionFromBegin(t *testing.T) {
	s := &scripted{installOK: true, sealResult: sealedOK()}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(s.events, "seal:new-transition") || outcome.Install.TransitionID != "new-transition" {
		t.Fatalf("events=%v outcome=%+v", s.events, outcome.Install)
	}
}

func contains(events []string, want string) bool {
	return slices.Contains(events, want)
}

func TestRunTransactionFailureRollsBackHostsThenPathThenAborts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		script *scripted
		want   []string
	}{
		{
			name: "path apply error", script: &scripted{applyErr: errors.New("apply boom")},
			want: []string{"apply-path", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
		{
			name: "host install error", script: &scripted{installErr: errors.New("install boom")},
			want: []string{"apply-path", "install-hosts", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
		{
			name: "host install ok=false without error", script: &scripted{installOK: false},
			want: []string{"apply-path", "install-hosts", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
		{
			name: "post-install shell plan error", script: &scripted{installOK: true, planShellErr: errors.New("shell boom"), planShellOnce: true},
			want: []string{"apply-path", "install-hosts", "plan-shell", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
		{
			name: "seal returns unsealed result", script: &scripted{installOK: true, sealResult: activationcontract.Result{OK: true, Sealed: false, Receipt: &activationcontract.Receipt{}}},
			want: []string{"apply-path", "install-hosts", "plan-shell", "seal:new-transition", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
		{
			name: "seal returns no receipt", script: &scripted{installOK: true, sealResult: activationcontract.Result{OK: true, Sealed: true}},
			want: []string{"apply-path", "install-hosts", "plan-shell", "seal:new-transition", "rollback-hosts", "rollback-path", "abort:new-transition"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			outcome, err := RunTransaction(t.Context(), TransactionRequest{}, tc.script, tc.script)
			if err == nil || outcome.Install == nil || outcome.Install.OK || outcome.Install.Committed {
				t.Fatalf("outcome=%+v err=%v", outcome.Install, err)
			}
			// Drop the preflight prefix; only the effect ordering after Begin matters.
			idx := -1
			for i, event := range tc.script.events {
				if event == "begin" {
					idx = i
				}
			}
			if got := tc.script.events[idx+1:]; !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("events after begin = %v, want %v", got, tc.want)
			}
			if contains(tc.script.events, "finalize-path") || contains(tc.script.events, "retired-state") {
				t.Fatalf("failed install must not finalize or delete retired state: %v", tc.script.events)
			}
		})
	}
}

func TestRunTransactionFailureNamesInstallerNotOKAndUnsealedReceipt(t *testing.T) {
	s := &scripted{installOK: false}
	_, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if err == nil || !strings.Contains(err.Error(), "native installer reported ok=false") {
		t.Fatalf("err = %v", err)
	}
	s = &scripted{installOK: true, sealResult: activationcontract.Result{}}
	_, err = RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if err == nil || !strings.Contains(err.Error(), "receipt was not sealed") {
		t.Fatalf("err = %v", err)
	}
}

func TestRunTransactionFailureJoinsRollbackAndAbortErrors(t *testing.T) {
	cause, hostErr, pathErr, abortErr := errors.New("apply boom"), errors.New("host rb"), errors.New("path rb"), errors.New("abort failed")
	s := &scripted{applyErr: cause, rollbackHost: hostErr, rollbackPath: pathErr, abortErr: abortErr}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	for _, want := range []error{cause, hostErr, pathErr, abortErr} {
		if !errors.Is(err, want) {
			t.Fatalf("joined error lost %v: %v", want, err)
		}
	}
	if outcome.Install == nil || outcome.Install.OK {
		t.Fatalf("outcome = %+v", outcome.Install)
	}
}

func TestRunTransactionSealStepFailureDefersAbortToCaller(t *testing.T) {
	s := &scripted{installErr: errors.New("install boom"), commandPath: &port.ManagedCommandPathResult{}}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{Step: "seal", TransitionID: "t-2"}, s, s)
	if err == nil {
		t.Fatal("want error")
	}
	if !outcome.Install.AbortRequired || !outcome.Install.CommandPath.AbortRequired {
		t.Fatalf("outcome = %+v", outcome.Install)
	}
	if !contains(s.events, "rollback-hosts") || !contains(s.events, "rollback-path") {
		t.Fatalf("rollbacks must still run: %v", s.events)
	}
	for _, event := range s.events {
		if strings.HasPrefix(event, "abort") {
			t.Fatalf("seal step must leave abort to caller: %v", s.events)
		}
	}
}

func TestRunTransactionFinalizeFailureKeepsCommitAndRetainsBackup(t *testing.T) {
	s := &scripted{installOK: true, sealResult: sealedOK(), finalizeErr: errors.New("rm backup denied"), commandPath: &port.ManagedCommandPathResult{}}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if err != nil {
		t.Fatalf("committed install must not fail on cleanup: %v", err)
	}
	got := outcome.Install
	if !got.Committed || !got.CommandPath.BackupRetained {
		t.Fatalf("outcome = %+v", got)
	}
	if last := got.Messages[len(got.Messages)-1]; !strings.Contains(last, "manual recovery") || !strings.Contains(last, "rm backup denied") {
		t.Fatalf("messages = %v", got.Messages)
	}
	if contains(s.events, "rollback-hosts") || contains(s.events, "abort:new-transition") {
		t.Fatalf("committed install rolled back: %v", s.events)
	}
	if s.events[len(s.events)-1] != "retired-state" {
		t.Fatalf("retired state must still be removed last: %v", s.events)
	}
}

func TestRunTransactionFinalizeFailureWithoutCommandPathStillCommits(t *testing.T) {
	s := &scripted{installOK: true, sealResult: sealedOK(), finalizeErr: errors.New("cleanup failed")}
	outcome, err := RunTransaction(t.Context(), TransactionRequest{}, s, s)
	if err != nil || !outcome.Install.Committed || outcome.Install.CommandPath != nil {
		t.Fatalf("outcome=%+v err=%v", outcome.Install, err)
	}
}
