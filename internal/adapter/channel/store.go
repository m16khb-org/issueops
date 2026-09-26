// Package channel adapts message storage, IDs, and clocks for the channel use case.
package channel

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	channelapp "issueops/internal/application/channel"
	channelcontract "issueops/internal/contract/channel"
)

const channelBucket = "channel_v1"

type StateDatabase interface {
	Get(bucket, id string) ([]byte, bool, error)
	Put(bucket, id string, data []byte) error
}

var (
	StateDir          func() string
	OpenStateDatabase func(dir string) (StateDatabase, error)
	GetExisting       func(dir, bucket, id string) ([]byte, bool, error)
	ListExisting      func(dir, bucket string) ([]string, error)
	channelNow        = time.Now
	channelWait       = time.Sleep
)

func StateRoot() string { return filepath.Join(StateDir(), "channel") }

func Send(req channelcontract.SendRequest) (channelcontract.SendResult, error) {
	return (channelapp.Service{Effects: channelEffects{}}).Send(req)
}

func Recv(req channelcontract.RecvRequest) (channelcontract.RecvResult, error) {
	return (channelapp.Service{Effects: channelEffects{}}).Recv(req)
}

type channelEffects struct{}

func (channelEffects) Open() (channelapp.Writer, error) {
	db, err := OpenStateDatabase(StateRoot())
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

func (channelEffects) NewID() string               { return newMessageID() }
func (channelEffects) Now() time.Time              { return channelNow() }
func (channelEffects) Wait(duration time.Duration) { channelWait(duration) }
func (channelEffects) ListIDs() ([]string, error)  { return ListExisting(StateRoot(), channelBucket) }

func (channelEffects) Get(id string) (channelcontract.Message, bool, error) {
	data, ok, err := GetExisting(StateRoot(), channelBucket, id)
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
func newMessageID() string {
	now := channelNow().UTC().UnixNano()
	var nonce [4]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		for i := range nonce {
			nonce[i] = byte(now >> (i * 8))
		}
	}
	return fmt.Sprintf("msg-%016x-%s", now, hex.EncodeToString(nonce[:]))
}
