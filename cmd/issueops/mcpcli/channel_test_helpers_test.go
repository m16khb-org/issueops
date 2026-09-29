package mcpcli

import (
	"issueops/internal/adapter/channel"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	channelapp "issueops/internal/application/channel"
	"path/filepath"
	"time"
)

func testChannelService() channelapp.Service { return testChannelServiceAt(statestore.StateDir()) }
func testChannelServiceAt(root string) channelapp.Service {
	return channelapp.Service{Effects: channel.Store{Root: filepath.Join(root, "channel"), Clock: time.Now, Sleep: time.Sleep, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting, OpenDatabase: func(dir string) (channel.StateDatabase, error) { return sqlstore.Open(dir) }}}
}
