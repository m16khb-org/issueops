// Package channel adapts message storage, IDs, and clocks for the channel use case.
package channel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	channelapp "issueops/internal/application/channel"
	channelcontract "issueops/internal/contract/channel"
)

const channelBucket = "channel_v1"

type StateDatabase interface {
	Put(bucket, id string, data []byte) error
}

type Store struct {
	Root         string
	OpenDatabase func(string) (StateDatabase, error)
	GetExisting  func(string, string, string) ([]byte, bool, error)
	ListExisting func(string, string) ([]string, error)
	Clock        func() time.Time
	Sleep        func(time.Duration)
}

func (store Store) Open() (channelapp.Writer, error) {
	db, err := store.OpenDatabase(store.Root)
	if err != nil {
		return nil, err
	}
	return messageWriter{db}, nil
}

type messageWriter struct{ db StateDatabase }

func (writer messageWriter) Write(message channelcontract.Message) error {
	data, err := json.MarshalIndent(message, "", "  ")
	if err != nil {
		return err
	}
	return writer.db.Put(channelBucket, message.ID, data)
}

func (store Store) NewID() string               { return newMessageID(store.Clock()) }
func (store Store) Now() time.Time              { return store.Clock() }
func (store Store) Wait(duration time.Duration) { store.Sleep(duration) }
func (store Store) ListIDs() ([]string, error)  { return store.ListExisting(store.Root, channelBucket) }
func (store Store) Get(id string) (channelcontract.Message, bool, error) {
	data, ok, err := store.GetExisting(store.Root, channelBucket, id)
	if err != nil || !ok {
		return channelcontract.Message{}, false, err
	}
	var message channelcontract.Message
	if err := json.Unmarshal(data, &message); err != nil {
		return channelcontract.Message{}, false, err
	}
	return message, true, nil
}

// Message IDs sort by creation time and use a nonce to distinguish same-nanosecond writes.
func newMessageID(timestamp time.Time) string {
	now := timestamp.UTC().UnixNano()
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		for i := range nonce {
			nonce[i] = byte(now >> (i * 8))
		}
	}
	return fmt.Sprintf("msg-%016x-%s", now, hex.EncodeToString(nonce[:]))
}
