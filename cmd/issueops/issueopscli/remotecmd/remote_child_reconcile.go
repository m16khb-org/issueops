package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"issueops/cmd/issueops/issueopscli/actorflags"
	app "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

func (command Command) runRemoteReconcileChild(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reconcile-child", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	operation := fs.String("operation-id", "", "original child operation ID")
	actor := actorflags.Add(fs)
	confirm := fs.Bool("confirm", false, "attach and adopt existing child; preview never writes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.ReconcileChild(context.Background(), command.Operations.IssueOpsStateRoot(), app.ChildReconcileCommand{ID: *id, OperationID: *operation, Confirm: *confirm, Actor: actor.Actor()}, actor.Observer(deps.observeNativeProcessAncestry))
	if *jsonOut {
		message := ""
		if err != nil {
			message = err.Error()
		}
		if e := deps.printJSON(struct {
			model.ChildReconcileResult
			Error string `json:"error,omitempty"`
		}{result, message}); e != nil {
			return e
		}
	} else if err == nil {
		fmt.Printf("child operation %s: %s (preview=%v)\n", result.OperationID, result.ChildURL, result.WouldAdopt)
	}
	return err
}
