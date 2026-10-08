package channelcli

import (
	"context"
	adapter "issueops/internal/adapter/channel"
	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/channel"
	model "issueops/internal/contract/channel"
	"path/filepath"
	"time"
)

func channelServiceForTest() app.Service {
	return app.Service{Effects: adapter.Store{Root: filepath.Join(statestore.StateDir(), "channel"), Clock: time.Now, Sleep: issueopsadapter.SleepWithContext, WalkExistingAfter: sqlstore.WalkExistingAfter, OpenDatabase: func(dir string) (adapter.StateDatabase, error) { return sqlstore.Open(dir) }}}
}
func adapterSend(req model.SendRequest) (model.SendResult, error) {
	return channelServiceForTest().Send(req)
}
func adapterRecv(ctx context.Context, req model.RecvRequest) (model.RecvResult, error) {
	return channelServiceForTest().Recv(ctx, req)
}
