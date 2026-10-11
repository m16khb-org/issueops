package issueopsexecution

import (
	"context"
	"errors"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestExecuteReportsMissingHandlerPerAction(t *testing.T) {
	cases := []struct {
		name string
		req  model.ExecutionActionRequest
		want error
		msg  string
	}{
		{name: "prepare", req: model.ExecutionActionRequest{Action: model.ExecutionActionPrepare}, want: model.ErrPrepareHandlerUnavailable},
		{name: "status", req: model.ExecutionActionRequest{Action: model.ExecutionActionStatus}, msg: "issueops execution status handler is not configured"},
		{name: "claim", req: model.ExecutionActionRequest{Action: model.ExecutionActionClaim}, want: model.ErrClaimHandlerUnavailable},
		{name: "release", req: model.ExecutionActionRequest{Action: model.ExecutionActionRelease}, want: model.ErrReleaseHandlerUnavailable},
		{name: "reseed", req: model.ExecutionActionRequest{Action: model.ExecutionActionReplace, ReplaceAction: model.ExecutionReplaceReseed}, want: model.ErrReseedHandlerUnavailable},
		{name: "replace", req: model.ExecutionActionRequest{Action: model.ExecutionActionReplace}, msg: "issueops execution replace handler is not configured"},
		{name: "resume", req: model.ExecutionActionRequest{Action: model.ExecutionActionResume, Confirm: true}, want: model.ErrResumeHandlerUnavailable},
		{name: "complete", req: model.ExecutionActionRequest{Action: model.ExecutionActionComplete}, want: model.ErrCompleteHandlerUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.req.ID = "cycle"
			_, err := Service{}.Execute(t.Context(), "state", tc.req, port.ExecutionActionDependencies{})
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) || (tc.msg != "" && err.Error() != tc.msg) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestExecuteResumeRequiresConfirmBeforeHandler(t *testing.T) {
	called := false
	_, err := Service{}.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: model.ExecutionActionResume}, port.ExecutionActionDependencies{
		Resume: func(context.Context, string, model.ExecutionResumeRequest) (model.ExecutionResumeResult, error) {
			called = true
			return model.ExecutionResumeResult{}, nil
		}})
	if err == nil || err.Error() != "execution resume requires confirm" || called {
		t.Fatalf("err=%v called=%v", err, called)
	}
}

func TestExecuteRejectsUnsupportedAction(t *testing.T) {
	got, err := Service{}.Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "cycle", Action: "bogus"}, port.ExecutionActionDependencies{})
	if got != nil || err == nil || err.Error() != `unsupported issueops execution action "bogus"` {
		t.Fatalf("got=%v err=%v", got, err)
	}
}

func TestExecuteReplaceRoutesReseedSeparately(t *testing.T) {
	var gotReseed, gotReplace bool
	deps := port.ExecutionActionDependencies{
		Reseed: func(_ context.Context, _ string, req model.ExecutionReseedRequest) (model.ExecutionReplaceResult, error) {
			gotReseed = req.Reason == "r"
			return model.ExecutionReplaceResult{OK: true}, nil
		},
		Replace: func(_ context.Context, _ string, req model.ExecutionReplaceRequest, _ port.ReplacementInvocation) (model.ExecutionReplaceResult, error) {
			gotReplace = req.Action == "finalize" && req.Reason == "r"
			return model.ExecutionReplaceResult{OK: true}, nil
		},
	}
	for _, action := range []string{model.ExecutionReplaceReseed, "finalize"} {
		if _, err := (Service{}).Execute(t.Context(), "state", model.ExecutionActionRequest{ID: "c", Action: model.ExecutionActionReplace, ReplaceAction: action, Reason: "r"}, deps); err != nil {
			t.Fatal(err)
		}
	}
	if !gotReseed || !gotReplace {
		t.Fatalf("reseed=%v replace=%v", gotReseed, gotReplace)
	}
}
