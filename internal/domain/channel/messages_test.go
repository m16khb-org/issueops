package channel

import (
	"errors"
	model "issueops/internal/contract/channel"
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

func TestRangeStartPreservesMissingCursor(t *testing.T) {
	req, err := NormalizeRecv(model.RecvRequest{Channel: "c", SinceID: " msg-b "})
	if err != nil || req.SinceID != "msg-b" {
		t.Fatalf("request=%+v error=%v", req, err)
	}
	for _, tc := range []struct {
		since  string
		exists bool
		want   string
	}{
		{"msg-b", true, "msg-b"}, {"missing", false, ""}, {"", false, ""},
	} {
		if got := RangeStart(tc.since, tc.exists); got != tc.want {
			t.Fatalf("since=%q exists=%v got=%q want=%q", tc.since, tc.exists, got, tc.want)
		}
	}
	now := time.Date(2026, 10, 8, 0, 0, 0, 0, time.UTC)
	if got := RetentionCutoff(now); !got.Equal(now.Add(-7 * 24 * time.Hour)) {
		t.Fatalf("retention cutoff=%v", got)
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
