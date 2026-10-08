package channel

import (
	"context"
	"errors"
	"testing"
	"time"

	channelcontract "issueops/internal/contract/channel"
)

type fakeWriter struct{ effects *fakeEffects }

func (w fakeWriter) Write(message channelcontract.Message) error {
	w.effects.events = append(w.effects.events, "write")
	w.effects.messages = append(w.effects.messages, message)
	return nil
}

func (w fakeWriter) PruneBefore(cutoff time.Time) error {
	w.effects.events = append(w.effects.events, "prune")
	w.effects.pruneCutoffs = append(w.effects.pruneCutoffs, cutoff)
	return w.effects.pruneErr
}

type fakeEffects struct {
	opens        int
	events       []string
	messages     []channelcontract.Message
	pruneCutoffs []time.Time
	pruneErr     error
	sinceSeen    []string
	now          time.Time
	wait         func(context.Context, time.Duration) error
}

func (f *fakeEffects) Open() (Writer, error) { f.opens++; return fakeWriter{f}, nil }
func (f *fakeEffects) NewID() string         { return "msg-new" }
func (f *fakeEffects) Now() time.Time        { return f.now }
func (f *fakeEffects) Wait(ctx context.Context, d time.Duration) error {
	return f.wait(ctx, d)
}
func (f *fakeEffects) MessagesAfter(ctx context.Context, since string) ([]channelcontract.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.sinceSeen = append(f.sinceSeen, since)
	return append([]channelcontract.Message(nil), f.messages...), nil
}

func TestSendValidatesBeforeOpeningStore(t *testing.T) {
	f := &fakeEffects{}
	service := Service{Effects: f}
	result, err := service.Send(channelcontract.SendRequest{Channel: " c ", Body: "x"})
	if !errors.Is(err, channelcontract.ErrFromRequired) || result.Channel != "c" || len(f.messages) != 0 || f.opens != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSendPrunesAtRetentionCutoffAfterWrite(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	f := &fakeEffects{now: now, pruneErr: errors.New("busy")}
	result, err := (Service{Effects: f}).Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "x"})
	if err != nil || !result.OK || result.Message.ID != "msg-new" {
		t.Fatalf("prune failure changed send result: result=%+v err=%v", result, err)
	}
	if len(f.events) != 2 || f.events[0] != "write" || f.events[1] != "prune" {
		t.Fatalf("events=%v", f.events)
	}
	if want := now.Add(-7 * 24 * time.Hour); len(f.pruneCutoffs) != 1 || !f.pruneCutoffs[0].Equal(want) {
		t.Fatalf("cutoffs=%v want=%v", f.pruneCutoffs, want)
	}
}

func TestRecvSelectsTargetChannelAfterTrimmedCursor(t *testing.T) {
	f := &fakeEffects{messages: []channelcontract.Message{{ID: "msg-1", Channel: "other"}, {ID: "msg-2", Channel: "target"}, {ID: "msg-3", Channel: "target"}}}
	result, err := (Service{Effects: f}).Recv(context.Background(), channelcontract.RecvRequest{Channel: "target", SinceID: " msg-0 ", Limit: 1})
	if err != nil || len(result.Messages) != 1 || result.Messages[0].ID != "msg-2" || result.LastID != "msg-2" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(f.sinceSeen) != 1 || f.sinceSeen[0] != "msg-0" {
		t.Fatalf("since=%v", f.sinceSeen)
	}
}

func TestRecvWaitPollsUntilTargetMessageArrives(t *testing.T) {
	f := &fakeEffects{now: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}
	f.messages = []channelcontract.Message{{ID: "msg-old", Channel: "other"}}
	f.wait = func(context.Context, time.Duration) error {
		f.messages = append(f.messages, channelcontract.Message{ID: "msg-new", Channel: "target"})
		return nil
	}
	result, err := (Service{Effects: f}).Recv(context.Background(), channelcontract.RecvRequest{Channel: "target", Wait: true, TimeoutSeconds: 1})
	if err != nil || !result.Waited || len(result.Messages) != 1 || result.Messages[0].ID != "msg-new" || len(f.sinceSeen) != 2 {
		t.Fatalf("recv=%+v polls=%d err=%v", result, len(f.sinceSeen), err)
	}
}

func TestRecvWaitReturnsWhenContextCancelled(t *testing.T) {
	f := &fakeEffects{now: time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)}
	ctx, cancel := context.WithCancel(context.Background())
	f.wait = func(ctx context.Context, d time.Duration) error {
		cancel()
		<-ctx.Done()
		return ctx.Err()
	}
	result, err := (Service{Effects: f}).Recv(ctx, channelcontract.RecvRequest{Channel: "target", Wait: true, TimeoutSeconds: 300})
	if !errors.Is(err, context.Canceled) || result.OK || result.TimedOut || result.Error == "" || len(f.sinceSeen) != 1 {
		t.Fatalf("result=%+v polls=%d err=%v", result, len(f.sinceSeen), err)
	}
}
