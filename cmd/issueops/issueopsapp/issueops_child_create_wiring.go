package issueopsapp

import (
	"context"
	adapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/provider"
	app "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
	"os"
)

func newChildCreator(root string) app.ChildCreator {
	return app.ChildCreator{
		Records: adapter.RemoteRecordStore{StateRoot: root},
		Resolve: func(name string) (app.ChildProvider, error) { return provider.Resolve(name) },
		Bodies:  app.NewTemplateBodyResolver(os.ReadFile),
		Authorize: func(_ context.Context, id string, actor model.IssueOpsActor) error {
			return adapter.ValidateIssueOpsMutationActor(root, id, actor)
		},
		Link: func(ctx context.Context, id, url, title string, actor model.IssueOpsActor) error {
			_, err := newIssueLinker(root).Child(ctx, id, url, title, &actor)
			return err
		},
	}
}
