package remotecmd

import (
	"context"
	"flag"
	"fmt"
	app "issueops/internal/application/issueopsremote"
	model "issueops/internal/contract/issueops"
)

func (command Command) runRemoteReconcileChild(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote reconcile-child", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	operation := fs.String("operation-id", "", "original child operation ID")
	host := fs.String("host", "", "native holder host")
	session := fs.String("session-id", "", "native holder session id")
	agent := fs.String("agent-id", "", "native holder agent id")
	cwd := fs.String("cwd", "", "canonical holder cwd")
	confirm := fs.Bool("confirm", false, "attach and adopt existing child; preview never writes")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := command.Operations.ReconcileChild(context.Background(), command.Operations.IssueOpsStateRoot(), app.ChildReconcileCommand{ID: *id, OperationID: *operation, Confirm: *confirm, Actor: model.IssueOpsActor{Host: *host, SessionID: *session, AgentID: *agent, CWD: *cwd}}, deps.observeNativeProcessAncestry)
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
