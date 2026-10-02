package authority

import (
	"context"
	"testing"

	contract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

func TestNativeOnlyVerifierRejectsBoundCapabilitiesWithoutPanic(t *testing.T) {
	h := newHarness()
	actor := h.nativeActor("caller", 101)
	use := h.issue(t, actor, "/repo")
	bound, _, err := h.service.Bind(context.Background(), use)
	if err != nil {
		t.Fatal(err)
	}
	span, err := h.service.BindSpan(bound, h.grants)
	if err != nil {
		t.Fatal(err)
	}
	nativeOnly := New(nil, h.procs, nil, nil, nil, nil)
	for name, ctx := range map[string]context.Context{"bound": bound, "locked": span} {
		t.Run(name, func(t *testing.T) {
			_, err := nativeOnly.Verify(ctx, model.NativeActor{})
			requireCode(t, err, contract.CodeInvalid)
		})
	}
}
