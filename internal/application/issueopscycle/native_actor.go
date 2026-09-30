package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type NativeProcessInspector func(model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error)

func NormalizeNativeActor(actor model.NativeActor, inspect NativeProcessInspector) (model.NativeActor, error) {
	actor, err := domain.NormalizeNativeActor(actor)
	if err != nil {
		return actor, err
	}
	status, observed, err := inspect(*actor.SessionProcess)
	if err != nil {
		return actor, err
	}
	return actor, domain.ValidateObservedNativeProcess(*actor.SessionProcess, status, observed)
}
