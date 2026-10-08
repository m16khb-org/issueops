package channel

import (
	"context"
	"encoding/json"
	"errors"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	channelapp "issueops/internal/application/channel"
	channelcontract "issueops/internal/contract/channel"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func storeForTest() Store {
	return Store{Root: filepath.Join(statestore.StateDir(), "channel"), Clock: time.Now, Sleep: func(ctx context.Context, d time.Duration) error { time.Sleep(d); return ctx.Err() }, WalkExistingAfter: sqlstore.WalkExistingAfter, OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }}
}

func TestSendRecvRoundTripAcrossChannelIsolation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "contract", From: "server", Body: "GET /users -> 200 {id,name}"}); err != nil {
		t.Fatalf("send 1: %v", err)
	}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "contract", From: "front", Body: "need email too"}); err != nil {
		t.Fatalf("send 2: %v", err)
	}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "other", From: "server", Body: "noise"}); err != nil {
		t.Fatalf("send 3: %v", err)
	}
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "contract"})
	if err != nil {
		t.Fatalf("recv: %v", err)
	}
	if !recv.OK || len(recv.Messages) != 2 {
		t.Fatalf("recv messages = %d, want 2 (cross-channel isolation): %+v", len(recv.Messages), recv)
	}
	if recv.Messages[0].From != "server" || recv.Messages[0].Body != "GET /users -> 200 {id,name}" {
		t.Fatalf("first message wrong: %+v", recv.Messages[0])
	}
	if recv.LastID != recv.Messages[1].ID {
		t.Fatalf("LastID must be the newest message: %q vs %q", recv.LastID, recv.Messages[1].ID)
	}
	for _, msg := range recv.Messages {
		if !msg.OK || msg.SchemaVersion != channelcontract.SchemaVersion || msg.ID == "" || msg.CreatedAt == "" {
			t.Fatalf("message contract fields incomplete: %+v", msg)
		}
	}
}

func TestRecvSinceIDReturnsOnlyNewMessages(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	first, _ := service.Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "1"})
	if _, err := service.Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "2"}); err != nil {
		t.Fatal(err)
	}
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c", SinceID: first.Message.ID})
	if err != nil {
		t.Fatalf("recv since: %v", err)
	}
	if len(recv.Messages) != 1 || recv.Messages[0].Body != "2" {
		t.Fatalf("since filter wrong: %+v", recv.Messages)
	}
	// 사라진 sinceID는 "처음부터"로 해석한다(위치를 알 근거가 없음).
	all, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c", SinceID: "msg-does-not-exist"})
	if err != nil {
		t.Fatalf("recv missing-since: %v", err)
	}
	if len(all.Messages) != 2 {
		t.Fatalf("missing sinceID should return all: %+v", all.Messages)
	}
}

func TestRecvLimitBoundsResult(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	for _, body := range []string{"1", "2", "3"} {
		if _, err := service.Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: body}); err != nil {
			t.Fatal(err)
		}
	}
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c", Limit: 2})
	if err != nil {
		t.Fatalf("recv limit: %v", err)
	}
	if len(recv.Messages) != 2 || recv.LastID != recv.Messages[1].ID {
		t.Fatalf("limit wrong: %+v", recv)
	}
}

func TestRecvWaitReturnsImmediatelyWhenMessageExists(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "c", From: "server", Body: "ready"}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c", Wait: true, TimeoutSeconds: 5})
	if err != nil {
		t.Fatalf("recv wait: %v", err)
	}
	if len(recv.Messages) != 1 || recv.TimedOut {
		t.Fatalf("existing message must return without waiting: %+v", recv)
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("wait with existing message took %v; should be immediate", elapsed)
	}
}

func TestRecvWaitTimesOutEmpty(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	now := time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)
	effects.Clock = func() time.Time { return now }
	effects.Sleep = func(_ context.Context, duration time.Duration) error { now = now.Add(duration); return nil }
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c", Wait: true, TimeoutSeconds: 1})
	if err != nil {
		t.Fatalf("recv wait timeout: %v", err)
	}
	if recv.OK != true || len(recv.Messages) != 0 || !recv.TimedOut {
		t.Fatalf("timeout result wrong: %+v", recv)
	}
}

func TestRecvWaitBlocksUntilConcurrentSend(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	waitStarted := make(chan struct{}, 1)
	resume := make(chan struct{})
	effects.Sleep = func(context.Context, time.Duration) error {
		waitStarted <- struct{}{}
		<-resume
		return nil
	}
	arrived := make(chan channelcontract.RecvResult, 1)
	go func() {
		recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "contract", Wait: true, TimeoutSeconds: 15})
		if err != nil {
			t.Errorf("wait recv: %v", err)
		}
		arrived <- recv
	}()
	select {
	case <-waitStarted:
	case <-time.After(time.Second):
		t.Fatal("receiver did not enter the exact wait seam")
	}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "contract", From: "server", Body: "contract v1"}); err != nil {
		t.Fatalf("send: %v", err)
	}
	close(resume)
	select {
	case recv := <-arrived:
		if len(recv.Messages) != 1 || recv.TimedOut || recv.Messages[0].Body != "contract v1" {
			t.Fatalf("waited recv wrong: %+v", recv)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("recv --wait did not observe the concurrent send")
	}
}

func TestMessagesAfterUsesExactCursorAndRestartsOnMissingCursor(t *testing.T) {
	effects := Store{Root: t.TempDir(), WalkExistingAfter: sqlstore.WalkExistingAfter, OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }}
	db, err := effects.OpenDatabase(effects.Root)
	if err != nil {
		t.Fatal(err)
	}
	ids := []string{"msg-000000000000000a-01", "msg-000000000000000b-01", "msg-000000000000000c-01"}
	for _, id := range ids {
		data, _ := jsonMarshal(channelcontract.Message{OK: true, SchemaVersion: channelcontract.SchemaVersion, ID: id, Channel: "c"})
		if err := db.Put(channelBucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Put(channelBucket, "msg-000000000000000d-01", []byte("{not json")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		since string
		want  []string
	}{
		{"", ids}, {ids[0], ids[1:]}, {ids[2], []string{}}, {"msg-000000000000000a-00", ids},
	} {
		messages, err := effects.MessagesAfter(context.Background(), tc.since)
		got := []string{}
		for _, message := range messages {
			got = append(got, message.ID)
		}
		if err != nil || !slices.Equal(got, tc.want) {
			t.Fatalf("since=%q got=%v want=%v err=%v", tc.since, got, tc.want, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := effects.MessagesAfter(ctx, ""); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled read error = %v", err)
	}
}

func TestSendPrunesMessagesPastRetention(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	effects := Store{Root: t.TempDir(), Clock: func() time.Time { return now }, WalkExistingAfter: sqlstore.WalkExistingAfter, OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }}
	service := channelapp.Service{Effects: effects}
	old := now.Add(-7*24*time.Hour - time.Nanosecond)
	boundary := now.Add(-7 * 24 * time.Hour)
	db, err := effects.OpenDatabase(effects.Root)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range []time.Time{old, boundary} {
		id := newMessageID(at)
		data, _ := jsonMarshal(channelcontract.Message{OK: true, SchemaVersion: channelcontract.SchemaVersion, ID: id, Channel: "c", Body: at.String()})
		if err := db.Put(channelBucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := service.Send(channelcontract.SendRequest{Channel: "c", From: "a", Body: "now"}); err != nil {
		t.Fatal(err)
	}
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "c"})
	if err != nil || len(recv.Messages) != 2 || recv.Messages[0].Body != boundary.String() || recv.Messages[1].Body != "now" {
		t.Fatalf("recv=%+v err=%v", recv, err)
	}
}

func TestSendValidation(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	cases := []struct {
		name string
		req  channelcontract.SendRequest
	}{
		{"empty channel", channelcontract.SendRequest{From: "a", Body: "x"}},
		{"empty from", channelcontract.SendRequest{Channel: "c", Body: "x"}},
		{"empty body", channelcontract.SendRequest{Channel: "c", From: "a"}},
	}
	for _, tc := range cases {
		result, err := service.Send(tc.req)
		if err == nil || result.OK {
			t.Fatalf("%s must fail: %+v %v", tc.name, result, err)
		}
	}
	if _, err := service.Recv(context.Background(), channelcontract.RecvRequest{}); err == nil {
		t.Fatal("recv without channel must fail")
	}
}

func jsonMarshal(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

func TestMessageIDsSortChronologically(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)

	effects := storeForTest()
	service := channelapp.Service{Effects: &effects}
	// 저장소에 직접 ID 순서를 강제하는 메시지를 써서 정렬 가정을 검증한다.
	ids := []string{"msg-000000000000000a-01", "msg-000000000000000b-01", "msg-000000000000000c-01"}
	db, err := effects.OpenDatabase(effects.Root)
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range ids {
		msg := channelcontract.Message{OK: true, SchemaVersion: channelcontract.SchemaVersion, ID: id, Channel: "sort", From: "a", Body: string(rune('0' + i)), CreatedAt: "2026-08-22T00:00:00Z"}
		data, _ := jsonMarshal(msg)
		if err := db.Put(channelBucket, id, data); err != nil {
			t.Fatal(err)
		}
	}
	recv, err := service.Recv(context.Background(), channelcontract.RecvRequest{Channel: "sort"})
	if err != nil {
		t.Fatal(err)
	}
	if len(recv.Messages) != 3 || recv.Messages[0].ID != ids[0] || recv.Messages[2].ID != ids[2] {
		t.Fatalf("chronological order broken: %+v", recv.Messages)
	}
}
