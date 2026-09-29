package mcpcli

import (
	"issueops/internal/adapter/channel"
	gatesadapter "issueops/internal/adapter/gates"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	policyadapter "issueops/internal/adapter/policy"
	channelapp "issueops/internal/application/channel"
	"path/filepath"
	"time"
)

// production wiring과 같은 channel/gates adapter 구현을 설치한다.
func init() {

	GatesCheck = gatesadapter.Check
	GatesInit = gatesadapter.Init
	GatesAbandon = gatesadapter.Abandon
	gatesadapter.EvaluateCommandPolicy = policyadapter.EvaluateCommandPolicy
	gatesadapter.RunCommand = policyadapter.RunCommand
}

func testChannelService() channelapp.Service { return testChannelServiceAt(statestore.StateDir()) }
func testChannelServiceAt(root string) channelapp.Service {
	return channelapp.Service{Effects: channel.Store{Root: filepath.Join(root, "channel"), Clock: time.Now, Sleep: time.Sleep, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting, OpenDatabase: func(dir string) (channel.StateDatabase, error) { return sqlstore.Open(dir) }}}
}
