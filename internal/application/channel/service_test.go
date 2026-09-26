package channel

import (
	"errors"
	"testing"
	"time"

	channelcontract "issueops/internal/contract/channel"
)

type fakeWriter struct{ messages *[]channelcontract.Message }

func (w fakeWriter) Write(message channelcontract.Message) error {
	*w.messages = append(*w.messages, message)
	return nil
}

type fakeEffects struct {
	messages []channelcontract.Message
	reads    map[string]int
	now      time.Time
	wait     func(time.Duration)
}

func (f *fakeEffects) Open() (Writer, error) { return fakeWriter{&f.messages}, nil }
func (f *fakeEffects) NewID() string         { return "msg-new" }
func (f *fakeEffects) Now() time.Time        { return f.now }
func (f *fakeEffects) Wait(d time.Duration)  { f.wait(d) }
func (f *fakeEffects) ListIDs() ([]string, error) {
	ids := make([]string, 0, len(f.messages))
	for _, message := range f.messages {
		ids = append(ids, message.ID)
	}
	return ids, nil
}
func (f *fakeEffects) Get(id string) (channelcontract.Message, bool, error) {
	f.reads[id]++
	for _, message := range f.messages {
		if message.ID == id {
			return message, true, nil
		}
	}
	return channelcontract.Message{}, false, nil
}

func TestRecvWaitKeepsObservedMessagesLocalToCall(t *testing.T) {
	f := &fakeEffects{reads: map[string]int{}, now: time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)}
	f.messages = []channelcontract.Message{{ID: "msg-old", Channel: "other"}}
	f.wait = func(time.Duration) {
		f.messages = append(f.messages, channelcontract.Message{ID: "msg-new", Channel: "target"})
	}
	service := Service{Effects: f}
	result, err := service.Recv(channelcontract.RecvRequest{Channel: "target", Wait: true, TimeoutSeconds: 1})
	if err != nil || len(result.Messages) != 1 || result.Messages[0].ID != "msg-new" || f.reads["msg-old"] != 1 {
		t.Fatalf("recv=%+v reads=%v err=%v", result, f.reads, err)
	}
	_, _ = service.Recv(channelcontract.RecvRequest{Channel: "target"})
	if f.reads["msg-old"] != 2 {
		t.Fatalf("observed messages leaked across calls: %v", f.reads)
	}
}

func TestSendValidatesBeforeOpeningStore(t *testing.T) {
	f := &fakeEffects{reads: map[string]int{}}
	service := Service{Effects: f}
	result, err := service.Send(channelcontract.SendRequest{Channel: " c ", Body: "x"})
	if !errors.Is(err, ErrFromRequired) || result.Channel != "c" || len(f.messages) != 0 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
