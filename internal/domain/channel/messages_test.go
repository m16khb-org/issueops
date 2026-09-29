package channel

import (
	"errors"
	model "issueops/internal/contract/channel"
	"reflect"
	"testing"
	"time"
)

func TestSendAdmissionPreservesBodyAndErrorPrecedence(t *testing.T) {
	for _, tc := range []struct {
		req  model.SendRequest
		want string
	}{
		{model.SendRequest{}, "channel_required"},
		{model.SendRequest{Channel: " c "}, "from_required"},
		{model.SendRequest{Channel: " c ", From: " sender ", Body: " \n "}, "body_required"},
	} {
		_, err := NormalizeSend(tc.req)
		if err == nil || err.Error() != tc.want {
			t.Fatalf("request=%+v error=%v", tc.req, err)
		}
		if tc.want == "from_required" && !errors.Is(err, model.ErrFromRequired) {
			t.Fatal("lost public error identity")
		}
	}
	req, err := NormalizeSend(model.SendRequest{Channel: " c ", From: " sender ", Body: "  exact body\n"})
	if err != nil || req.Channel != "c" || req.From != "sender" || req.Body != "  exact body\n" {
		t.Fatalf("request=%+v error=%v", req, err)
	}
}

func TestIDsAfterPreserveMissingCursor(t *testing.T) {
	for _, tc := range []struct {
		since string
		want  []string
	}{
		{" msg-b ", []string{"msg-c"}}, {"missing", []string{"msg-a", "msg-b", "msg-c"}},
	} {
		got := IDsAfter([]string{"msg-c", "msg-a", "msg-b"}, tc.since)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("since=%q got=%v want=%v", tc.since, got, tc.want)
		}
	}
}

func TestChannelSelectionAndPollDeadline(t *testing.T) {
	req := model.RecvRequest{Channel: "target", Limit: 1}
	if include, stop := SelectReceived(req, 1, model.Message{Channel: "other"}); include || stop {
		t.Fatal("another channel consumed the result limit")
	}
	if include, stop := SelectReceived(req, 1, model.Message{Channel: "target"}); include || !stop {
		t.Fatal("limit ignored")
	}
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	if got := PollDelay(now.Add(20*time.Millisecond), now); got != 20*time.Millisecond {
		t.Fatalf("poll=%v", got)
	}
	if WaitTimeout(0) != 300*time.Second || WaitTimeout(-1) != 300*time.Second || WaitTimeout(2) != 2*time.Second {
		t.Fatal("wait timeout defaults changed")
	}
}
