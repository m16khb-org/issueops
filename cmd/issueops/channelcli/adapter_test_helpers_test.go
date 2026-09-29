package channelcli

import (
	adapter "issueops/internal/adapter/channel"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/channel"
	model "issueops/internal/contract/channel"
	"path/filepath"
	"time"
)

func channelServiceForTest() app.Service {
	return app.Service{Effects: adapter.Store{Root: filepath.Join(statestore.StateDir(), "channel"), Clock: time.Now, Sleep: time.Sleep, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting, OpenDatabase: func(dir string) (adapter.StateDatabase, error) { return sqlstore.Open(dir) }}}
}
func adapterSend(req model.SendRequest) (model.SendResult, error) {
	return channelServiceForTest().Send(req)
}
func adapterRecv(req model.RecvRequest) (model.RecvResult, error) {
	return channelServiceForTest().Recv(req)
}
