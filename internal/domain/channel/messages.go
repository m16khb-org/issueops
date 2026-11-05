// Package channel holds the domain rules for the session message channel: request
// normalization, message construction, wait/poll timing, and retention. It performs no I/O.
package channel

import (
	"errors"
	"fmt"
	"strings"
	"time"

	model "issueops/internal/contract/channel"
)

// Probe: `channel send --json` answered bare codes; each now names the
// field on both surfaces. The code stays the message prefix.
var errChannelRequired = errors.New("channel_required: name the channel both sides use (CLI --channel, MCP channel)")

// NormalizeSend trims the channel and sender of a send request and rejects empty
// channel, sender (wrapping ErrFromRequired), or body and any origin other than CLI or MCP.
func NormalizeSend(req model.SendRequest) (model.SendRequest, error) {
	req.Channel = strings.TrimSpace(req.Channel)
	req.From = strings.TrimSpace(req.From)
	switch {
	case req.Channel == "":
		return req, errChannelRequired
	case req.From == "":
		return req, fmt.Errorf("%w: name the sender, e.g. this session id (CLI --from, MCP from)", model.ErrFromRequired)
	case strings.TrimSpace(req.Body) == "":
		return req, errors.New("body_required: the message text is empty (CLI --message, MCP body)")
	case req.Origin != model.OriginCLI && req.Origin != model.OriginMCP:
		return req, errors.New("origin_unknown")
	}
	return req, nil
}

// NormalizeRecv trims the channel and since cursor of a recv request and requires a channel.
func NormalizeRecv(req model.RecvRequest) (model.RecvRequest, error) {
	req.Channel = strings.TrimSpace(req.Channel)
	req.SinceID = strings.TrimSpace(req.SinceID)
	if req.Channel == "" {
		return req, errChannelRequired
	}
	return req, nil
}

// SentMessage builds the stored Message for a validated send request, stamping id and the
// UTC creation time from now.
func SentMessage(req model.SendRequest, id string, now time.Time) model.Message {
	return model.Message{OK: true, SchemaVersion: model.SchemaVersion, ID: id, Channel: req.Channel, From: req.From, Origin: req.Origin, Body: req.Body, CreatedAt: now.UTC().Format(time.RFC3339Nano)}
}

// WaitTimeout converts a recv wait in seconds to a duration, using the default timeout
// for non-positive values.
func WaitTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = model.DefaultWaitTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

// PollDelay returns how long recv sleeps before the next poll: the fixed poll interval,
// shortened to the time remaining until deadline.
func PollDelay(deadline, now time.Time) time.Duration {
	poll := time.Millisecond * time.Duration(model.WaitPollIntervalMS)
	if remaining := deadline.Sub(now); remaining < poll {
		return remaining
	}
	return poll
}

// Received marks result OK, attaches messages, and records the last message ID as the
// cursor for the next recv.
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

// retentionPeriod is how long a sent message stays readable before send prunes it.
const retentionPeriod = 7 * 24 * time.Hour

// RetentionCutoff returns the instant before which sent messages are pruned.
func RetentionCutoff(now time.Time) time.Time { return now.Add(-retentionPeriod) }

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
