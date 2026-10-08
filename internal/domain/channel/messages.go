package channel

import (
	"errors"
	"strings"
	"time"

	model "issueops/internal/contract/channel"
)

func NormalizeSend(req model.SendRequest) (model.SendRequest, error) {
	req.Channel = strings.TrimSpace(req.Channel)
	req.From = strings.TrimSpace(req.From)
	switch {
	case req.Channel == "":
		return req, errors.New("channel_required")
	case req.From == "":
		return req, model.ErrFromRequired
	case strings.TrimSpace(req.Body) == "":
		return req, errors.New("body_required")
	}
	return req, nil
}
func NormalizeRecv(req model.RecvRequest) (model.RecvRequest, error) {
	req.Channel = strings.TrimSpace(req.Channel)
	req.SinceID = strings.TrimSpace(req.SinceID)
	if req.Channel == "" {
		return req, errors.New("channel_required")
	}
	return req, nil
}
func SentMessage(req model.SendRequest, id string, now time.Time) model.Message {
	return model.Message{OK: true, SchemaVersion: model.SchemaVersion, ID: id, Channel: req.Channel, From: req.From, Body: req.Body, CreatedAt: now.UTC().Format(time.RFC3339Nano)}
}
func WaitTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = model.DefaultWaitTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}
func PollDelay(deadline, now time.Time) time.Duration {
	poll := time.Millisecond * time.Duration(model.WaitPollIntervalMS)
	if remaining := deadline.Sub(now); remaining < poll {
		return remaining
	}
	return poll
}
func Received(result model.RecvResult, messages []model.Message) model.RecvResult {
	result.OK = true
	result.Messages = messages
	if len(messages) > 0 {
		result.LastID = messages[len(messages)-1].ID
	}
	return result
}

// RangeStart returns the id after which recv reads. A cursor that is no longer
// stored has no known position, so recv restarts from the beginning.
func RangeStart(since string, sinceExists bool) string {
	if !sinceExists {
		return ""
	}
	return since
}

// RetentionPeriod is how long a sent message stays readable before send prunes it.
const RetentionPeriod = 7 * 24 * time.Hour

func RetentionCutoff(now time.Time) time.Time { return now.Add(-RetentionPeriod) }

// SelectReceived checks channel membership before applying the result limit.
func SelectReceived(req model.RecvRequest, count int, message model.Message) (include, stop bool) {
	if message.Channel != req.Channel {
		return false, false
	}
	if req.Limit > 0 && count >= req.Limit {
		return false, true
	}
	return true, false
}
