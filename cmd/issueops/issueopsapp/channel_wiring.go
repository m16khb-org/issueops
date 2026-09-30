package issueopsapp

import (
	"issueops/cmd/issueops/channelcli"
	channeladapter "issueops/internal/adapter/channel"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	channelapp "issueops/internal/application/channel"
	"path/filepath"
	"time"
)

func newChannelService(stateRoot string) channelapp.Service {
	return channelapp.Service{Effects: channeladapter.Store{
		Root: filepath.Join(stateRoot, "channel"), Clock: time.Now, Sleep: time.Sleep,
		OpenDatabase: func(dir string) (channeladapter.StateDatabase, error) { return sqlstore.Open(dir) },
		GetExisting:  sqlstore.GetExisting, ListExisting: sqlstore.ListExisting,
	}}
}
func channelDependencies() channelcli.Dependencies {
	service := newChannelService(statestore.StateDir())
	return channelcli.Dependencies{Send: service.Send, Recv: service.Recv}
}
