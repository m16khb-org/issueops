package channel

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	channelcontract "issueops/internal/contract/channel"
)

var ErrFromRequired = errors.New("from_required")

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
	result := channelcontract.SendResult{SchemaVersion: channelcontract.SchemaVersion, Channel: strings.TrimSpace(req.Channel)}
	req.Channel = strings.TrimSpace(req.Channel)
	req.From = strings.TrimSpace(req.From)
	var err error
	switch {
	case req.Channel == "":
		err = errors.New("channel_required")
	case req.From == "":
		err = ErrFromRequired
	case strings.TrimSpace(req.Body) == "":
		err = errors.New("body_required")
	}
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	writer, err := service.Effects.Open()
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	msg := channelcontract.Message{
		OK:            true,
		SchemaVersion: channelcontract.SchemaVersion,
		ID:            service.Effects.NewID(),
		Channel:       req.Channel,
		From:          req.From,
		Body:          req.Body,
		CreatedAt:     service.Effects.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := writer.Write(msg); err != nil {
		result.Error = err.Error()
		return result, err
	}
	result.OK = true
	result.Message = msg
	return result, nil
}

func (service Service) Recv(req channelcontract.RecvRequest) (channelcontract.RecvResult, error) {
	result := channelcontract.RecvResult{SchemaVersion: channelcontract.SchemaVersion, Channel: strings.TrimSpace(req.Channel)}
	req.Channel = strings.TrimSpace(req.Channel)
	if req.Channel == "" {
		result.Error = "channel_required"
		return result, fmt.Errorf("channel_required")
	}
	if !req.Wait {
		messages, err := service.read(req, nil)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		fillResult(&result, messages)
		return result, nil
	}
	timeout := channelcontract.DefaultWaitTimeoutSeconds
	if req.TimeoutSeconds > 0 {
		timeout = req.TimeoutSeconds
	}
	result.Waited = true
	deadline := service.Effects.Now().Add(time.Duration(timeout) * time.Second)
	observed := map[string]struct{}{}
	for {
		messages, err := service.read(req, observed)
		if err != nil {
			result.Error = err.Error()
			return result, err
		}
		if len(messages) > 0 {
			fillResult(&result, messages)
			return result, nil
		}
		if !service.Effects.Now().Before(deadline) {
			result.OK = true
			result.TimedOut = true
			return result, nil
		}
		remaining := deadline.Sub(service.Effects.Now())
		poll := time.Millisecond * time.Duration(channelcontract.WaitPollIntervalMS)
		if remaining < poll {
			poll = remaining
		}
		if poll > 0 {
			service.Effects.Wait(poll)
		}
	}
}

func fillResult(result *channelcontract.RecvResult, messages []channelcontract.Message) {
	result.OK = true
	result.Messages = messages
	if len(messages) > 0 {
		result.LastID = messages[len(messages)-1].ID
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
	sort.Strings(ids)
	startAt := 0
	if trimmed := strings.TrimSpace(req.SinceID); trimmed != "" {
		for i, id := range ids {
			if id == trimmed {
				startAt = i + 1
				break
			}
		}
	}
	messages := []channelcontract.Message{}
	for _, id := range ids[startAt:] {
		if _, alreadyObserved := observed[id]; alreadyObserved {
			continue
		}
		msg, ok, err := service.Effects.Get(id)
		if err != nil || !ok {
			continue
		}
		if observed != nil {
			observed[id] = struct{}{}
		}
		if msg.Channel != req.Channel {
			continue
		}
		if req.Limit > 0 && len(messages) >= req.Limit {
			break
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
