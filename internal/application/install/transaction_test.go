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

type transactionEffects struct {
	events     *[]string
	installErr error
	sealErr    error
}

func (effects transactionEffects) event(name string) { *effects.events = append(*effects.events, name) }
func (effects transactionEffects) PreparePath(*port.NativeInstallResult, port.NativeInstallRequest, string, string) (PathTransaction, error) {
	effects.event("prepare-path")
	return effects, nil
}
func (effects transactionEffects) Install(request port.NativeInstallRequest) (port.NativeInstallResult, error) {
	if request.DryRun {
		effects.event("plan-hosts")
	} else {
		effects.event("install-hosts")
	}
	return port.NativeInstallResult{OK: effects.installErr == nil}, effects.installErr
}
func (effects transactionEffects) PlanShell(*port.NativeInstallResult, port.NativeInstallRequest, string) error {
	effects.event("plan-shell")
	return nil
}
func (effects transactionEffects) PrepareHost(port.NativeInstallResult) (HostTransaction, error) {
	effects.event("snapshot-hosts")
	return effects, nil
}
func (effects transactionEffects) AppendUpstream(*port.NativeInstallResult, string, bool) {
	effects.event("upstream")
}
func (effects transactionEffects) RemoveRetiredState(result *port.NativeInstallResult, dryRun bool) {
	effects.event("retired-state")
	result.Messages = append(result.Messages, map[bool]string{true: "would remove retired", false: "removed retired"}[dryRun])
}
func (effects transactionEffects) Apply(*port.NativeInstallResult) error {
	effects.event("apply-path")
	return nil
}
func (effects transactionEffects) Rollback(*port.NativeInstallResult) error {
	effects.event("rollback-path")
	return nil
}
func (effects transactionEffects) Finalize(*port.NativeInstallResult) error {
	effects.event("finalize-path")
	return nil
}
func (effects transactionEffects) RollbackHosts() error { effects.event("rollback-hosts"); return nil }
func (effects transactionEffects) Begin(context.Context, activationcontract.Request) (activationcontract.Result, error) {
	effects.event("begin")
	return activationcontract.Result{TransitionID: "transition", BinarySHA256: "digest"}, nil
}
func (effects transactionEffects) Seal(context.Context, activationcontract.Request) (activationcontract.Result, error) {
	effects.event("seal")
	if effects.sealErr != nil {
		return activationcontract.Result{}, effects.sealErr
	}
	return activationcontract.Result{OK: true, Sealed: true, Receipt: &activationcontract.Receipt{}}, nil
}
func (effects transactionEffects) Abort(context.Context, activationcontract.Request) (activationcontract.Result, error) {
	effects.event("abort")
	return activationcontract.Result{TransitionID: "transition"}, nil
}

func TestRunTransactionPreservesActivationAndRollbackOrder(t *testing.T) {
	for _, tc := range []struct {
		name    string
		request TransactionRequest
		sealErr error
		want    []string
		wantErr bool
	}{
		{name: "dry run", request: TransactionRequest{Install: port.NativeInstallRequest{DryRun: true}}, want: []string{"prepare-path", "plan-hosts", "upstream", "retired-state"}},
		{name: "commit", request: TransactionRequest{}, want: []string{"prepare-path", "plan-hosts", "plan-shell", "snapshot-hosts", "begin", "apply-path", "install-hosts", "plan-shell", "seal", "upstream", "finalize-path", "retired-state"}},
		{name: "seal failure", request: TransactionRequest{}, sealErr: errors.New("seal failed"), wantErr: true, want: []string{"prepare-path", "plan-hosts", "plan-shell", "snapshot-hosts", "begin", "apply-path", "install-hosts", "plan-shell", "seal", "rollback-hosts", "rollback-path", "abort"}},
		{name: "begin only", request: TransactionRequest{Step: "begin"}, want: []string{"prepare-path", "plan-hosts", "plan-shell", "snapshot-hosts", "begin"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			effects := transactionEffects{events: &events, sealErr: tc.sealErr}
			outcome, err := RunTransaction(context.Background(), tc.request, effects, effects)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(events, tc.want) {
				t.Fatalf("events = %v, want %v", events, tc.want)
			}
			if tc.name == "commit" && (outcome.Install == nil || !outcome.Install.Committed) {
				t.Fatalf("commit outcome = %+v", outcome)
			}
			if tc.name == "seal failure" && (outcome.Install == nil || outcome.Install.OK) {
				t.Fatalf("failure outcome = %+v", outcome)
			}
			if tc.name == "begin only" && outcome.Activation == nil {
				t.Fatalf("begin outcome = %+v", outcome)
			}
		})
	}
}

func TestRunTransactionRemovesRetiredStateOnlyAfterFinalize(t *testing.T) {
	for _, tc := range []struct {
		name        string
		request     TransactionRequest
		sealErr     error
		wantMessage string
	}{
		{name: "dry run plans", request: TransactionRequest{Install: port.NativeInstallRequest{DryRun: true}}, wantMessage: "would remove retired"},
		{name: "commit removes", request: TransactionRequest{}, wantMessage: "removed retired"},
		{name: "failed seal keeps", request: TransactionRequest{}, sealErr: errors.New("seal failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			effects := transactionEffects{events: &events, sealErr: tc.sealErr}
			outcome, _ := RunTransaction(context.Background(), tc.request, effects, effects)
			messages := []string{}
			if outcome.Install != nil {
				messages = outcome.Install.Messages
			}
			removed := slices.Index(events, "retired-state")
			if tc.wantMessage == "" {
				if removed >= 0 || slices.ContainsFunc(messages, func(m string) bool { return strings.Contains(m, "retired") }) {
					t.Fatalf("retired state touched after failed install: events=%v messages=%v", events, messages)
				}
				return
			}
			if removed != len(events)-1 || !slices.Contains(messages, tc.wantMessage) {
				t.Fatalf("events=%v messages=%v", events, messages)
			}
			if finalized := slices.Index(events, "finalize-path"); !tc.request.Install.DryRun && finalized > removed {
				t.Fatalf("retired state removed before finalize: %v", events)
			}
		})
	}
}
