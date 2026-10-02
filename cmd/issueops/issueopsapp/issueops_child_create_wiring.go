package issueopsapp

import (
	"context"
	"fmt"
	adapter "issueops/internal/adapter/issueops"
	authorization "issueops/internal/adapter/outbound/issueopsauthorization"
	"issueops/internal/adapter/provider"
	cycleapp "issueops/internal/application/issueopscycle"
	app "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"os"
	"time"
)

func newChildCreator(root string) app.ChildCreator {
	return app.ChildCreator{
		Records: adapter.RemoteRecordStore{StateRoot: root},
		Resolve: func(name string) (app.ChildProvider, error) { return provider.Resolve(name) },
		Bodies:  app.NewTemplateBodyResolver(os.ReadFile),
		Authorize: func(ctx context.Context, id string, actor model.IssueOpsActor) error {
			return adapter.ValidateIssueOpsMutationActor(ctx, root, id, actor, issueOpsActorVerifier())
		},
		Intents:        &app.ChildCreateIntents{Store: adapter.RemoteRecordStore{StateRoot: root}, Authority: cycleapp.NewMutationAuthority(authorization.CanonicalPaths{}.Same, issueOpsActorVerifier()), Now: time.Now},
		NewOperationID: adapter.RemotePublicationObserver{}.NewOperationID,
	}
}

func newChildReconciler(root string) app.ChildReconciler {
	creator := newChildCreator(root)
	return app.ChildReconciler{Records: creator.Records, Intents: creator.Intents, Resolve: func(name string) (port.IssueProviderChildCreateRecovery, error) {
		resolved, err := provider.Resolve(name)
		if err != nil {
			return nil, err
		}
		recovery, ok := resolved.(port.IssueProviderChildCreateRecovery)
		if !ok {
			return nil, fmt.Errorf("provider lacks child recovery")
		}
		return recovery, nil
	}}
}
