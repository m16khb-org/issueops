// Package channel adapts message storage, IDs, and clocks for the channel use case.
package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	channelapp "issueops/internal/application/channel"
	channelcontract "issueops/internal/contract/channel"
	channeldomain "issueops/internal/domain/channel"
	"issueops/internal/port"
)

const channelBucket = "channel_v1"

// StateDatabase is the slice of the state database the channel store writes:
// storing a message and pruning messages older than an ID.
type StateDatabase interface {
	Put(bucket, id string, data []byte) error
	DeleteBefore(ctx context.Context, bucket, beforeID string) error
}

// Store adapts the channel state database, message IDs, and clock to the channel
// use case. Root is the state root; the function fields are injected so tests
// can replace the database, clock, and sleep.
type Store struct {
	Root              string
	OpenDatabase      func(string) (StateDatabase, error)
	WalkExistingAfter func(ctx context.Context, dir, bucket, cursor string, start func(bool) string, visit func(port.RecordRow) error) error
	Clock             func() time.Time
	Sleep             func(context.Context, time.Duration) error
}

// Open opens the state database under store.Root and returns a writer that
// stores messages in it.
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

// PruneBefore deletes messages created before cutoff. Message IDs start with
// the creation time, so the bare time prefix bounds every older ID.
func (writer messageWriter) PruneBefore(cutoff time.Time) error {
	return writer.db.DeleteBefore(context.Background(), channelBucket, messageIDPrefix(cutoff))
}

// NewID returns a message ID derived from the store clock.
func (store Store) NewID() string { return newMessageID(store.Clock()) }

// Now returns the current time from the store clock.
func (store Store) Now() time.Time { return store.Clock() }

// Wait blocks for duration using the store's sleep function, returning early with
// an error when ctx ends.
func (store Store) Wait(ctx context.Context, duration time.Duration) error {
	return store.Sleep(ctx, duration)
}

// MessagesAfter reads every message after the domain-resolved cursor on one
// read-only connection. Rows that do not decode are skipped and read again on
// the next call.
func (store Store) MessagesAfter(ctx context.Context, since string) ([]channelcontract.Message, error) {
	messages := []channelcontract.Message{}
	start := func(sinceExists bool) string { return channeldomain.RangeStart(since, sinceExists) }
	err := store.WalkExistingAfter(ctx, store.Root, channelBucket, since, start, func(row port.RecordRow) error {
		var message channelcontract.Message
		if json.Unmarshal(row.Data, &message) == nil {
			messages = append(messages, message)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return messages, nil
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
	return messageIDPrefix(timestamp) + "-" + hex.EncodeToString(nonce[:])
}

func messageIDPrefix(timestamp time.Time) string {
	return fmt.Sprintf("msg-%016x", timestamp.UTC().UnixNano())
}
