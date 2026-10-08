package channel

import (
	"context"
	"errors"
	"io/fs"
	"time"

	channelcontract "issueops/internal/contract/channel"
	channeldomain "issueops/internal/domain/channel"
)

type Writer interface {
	Write(channelcontract.Message) error
	PruneBefore(time.Time) error
}

type Effects interface {
	Open() (Writer, error)
	NewID() string
	Now() time.Time
	Wait(context.Context, time.Duration) error
	// MessagesAfter returns decodable stored messages in id order after the
	// range start that channeldomain.RangeStart resolves for since.
	MessagesAfter(ctx context.Context, since string) ([]channelcontract.Message, error)
}

type Service struct{ Effects Effects }

func (service Service) Send(req channelcontract.SendRequest) (channelcontract.SendResult, error) {
	req, err := channeldomain.NormalizeSend(req)
	result := channelcontract.SendResult{SchemaVersion: channelcontract.SchemaVersion, Channel: req.Channel}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	writer, err := service.Effects.Open()
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	now := service.Effects.Now()
	msg := channeldomain.SentMessage(req, service.Effects.NewID(), now)
	if err := writer.Write(msg); err != nil {
		result.Error = err.Error()
		return result, err
	}
	// Retention is housekeeping after a durable write: a failed prune must not
	// report the stored message as unsent, and the next send retries it.
	_ = writer.PruneBefore(channeldomain.RetentionCutoff(now))
	result.OK = true
	result.Message = msg
	return result, nil
}

func (service Service) Recv(ctx context.Context, req channelcontract.RecvRequest) (channelcontract.RecvResult, error) {
	req, err := channeldomain.NormalizeRecv(req)
	result := channelcontract.RecvResult{SchemaVersion: channelcontract.SchemaVersion, Channel: req.Channel}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	if !req.Wait {
		messages, err := service.read(ctx, req)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result = channeldomain.Received(result, messages)
		return result, nil
	}
	result.Waited = true
	deadline := service.Effects.Now().Add(channeldomain.WaitTimeout(req.TimeoutSeconds))
	for {
		messages, err := service.read(ctx, req)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		if len(messages) > 0 {
			result = channeldomain.Received(result, messages)
			return result, nil
		}
		if !service.Effects.Now().Before(deadline) {
			result.OK = true
			result.TimedOut = true
			return result, nil
		}
		poll := channeldomain.PollDelay(deadline, service.Effects.Now())
		if poll > 0 {
			if err := service.Effects.Wait(ctx, poll); err != nil {
				result.Error = err.Error()
				return result, err
			}
		}
	}
}

func (service Service) read(ctx context.Context, req channelcontract.RecvRequest) ([]channelcontract.Message, error) {
	stored, err := service.Effects.MessagesAfter(ctx, req.SinceID)
	if errors.Is(err, fs.ErrNotExist) {
		return []channelcontract.Message{}, nil
	}
	if err != nil {
		return nil, err
	}
	messages := []channelcontract.Message{}
	for _, msg := range stored {
		include, stop := channeldomain.SelectReceived(req, len(messages), msg)
		if stop {
			break
		}
		if include {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}
