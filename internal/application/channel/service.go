package channel

import (
	"errors"
	"io/fs"
	"time"

	channelcontract "issueops/internal/contract/channel"
	channeldomain "issueops/internal/domain/channel"
)

type Writer interface {
	Write(channelcontract.Message) error
}

type Effects interface {
	Open() (Writer, error)
	NewID() string
	Now() time.Time
	Wait(time.Duration)
	ListIDs() ([]string, error)
	Get(string) (channelcontract.Message, bool, error)
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
	msg := channeldomain.SentMessage(req, service.Effects.NewID(), service.Effects.Now())
	if err := writer.Write(msg); err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.OK = true
	result.Message = msg
	return result, nil
}

func (service Service) Recv(req channelcontract.RecvRequest) (channelcontract.RecvResult, error) {
	req, err := channeldomain.NormalizeRecv(req)
	result := channelcontract.RecvResult{SchemaVersion: channelcontract.SchemaVersion, Channel: req.Channel}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	if !req.Wait {
		messages, err := service.read(req, nil)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		result = channeldomain.Received(result, messages)
		return result, nil
	}
	result.Waited = true
	deadline := service.Effects.Now().Add(channeldomain.WaitTimeout(req.TimeoutSeconds))
	observed := map[string]struct{}{}
	for {
		messages, err := service.read(req, observed)
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
			service.Effects.Wait(poll)
		}
	}
}

func (service Service) read(req channelcontract.RecvRequest, observed map[string]struct{}) ([]channelcontract.Message, error) {
	ids, err := service.Effects.ListIDs()
	if errors.Is(err, fs.ErrNotExist) {
		return []channelcontract.Message{}, nil
	}
	if err != nil {
		return nil, err
	}
	messages := []channelcontract.Message{}
	for _, id := range channeldomain.IDsAfter(ids, req.SinceID) {
		if _, seen := observed[id]; seen {
			continue
		}
		msg, ok, err := service.Effects.Get(id)
		if err != nil || !ok {
			continue
		}
		if observed != nil {
			observed[id] = struct{}{}
		}
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
