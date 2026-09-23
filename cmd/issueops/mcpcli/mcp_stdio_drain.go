package mcpcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"sync"
	"time"
)

// host나 셸 파이프라인이 요청을 보낸 직후 stdin을 닫아도, 이미 받은 요청의 응답은
// 모두 돌려준 뒤 세션을 닫는다. go-sdk는 입력 EOF에서 연결을 닫으면서 처리 중인
// 요청을 버리므로, 입력 쪽 EOF를 in-flight 요청이 없어질 때까지 미룬다. 예전 daemon
// proxy가 host EOF 뒤에도 응답을 끝까지 돌려주던 동작(8a729a33)을 in-process
// stdio 경로에서도 지킨다.

// mcpInputEOFDrainLimit는 입력 EOF 뒤 응답을 기다리는 상한이다. 짝을 찾지 못한
// 요청이 있어도 host가 떠난 뒤 프로세스가 무기한 남지 않게 한다. 긴 도구 호출도
// 끝낼 수 있도록 넉넉하게 둔다.
const mcpInputEOFDrainLimit = 10 * time.Minute

type inflightRequests struct {
	mu      sync.Mutex
	pending map[string]struct{}
	changed chan struct{}
}

func newInflightRequests() *inflightRequests {
	return &inflightRequests{pending: map[string]struct{}{}, changed: make(chan struct{})}
}

// observeInbound는 host가 보낸 요청을 대기 목록에 넣고, 취소 알림을 받은 요청은
// 뺀다. 취소된 요청에는 응답이 오지 않을 수 있다.
func (inflight *inflightRequests) observeInbound(line []byte) {
	for _, message := range decodeJSONRPCMessages(line) {
		switch {
		case message.Method == "notifications/cancelled":
			var params struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			if json.Unmarshal(message.Params, &params) == nil {
				inflight.forget(jsonRPCIDKey(params.RequestID))
			}
		case message.Method != "" && jsonRPCIDKey(message.ID) != "":
			inflight.mu.Lock()
			inflight.pending[jsonRPCIDKey(message.ID)] = struct{}{}
			inflight.mu.Unlock()
		}
	}
}

// observeOutbound는 서버가 쓴 응답(result 또는 error)의 요청을 목록에서 뺀다.
func (inflight *inflightRequests) observeOutbound(line []byte) {
	for _, message := range decodeJSONRPCMessages(line) {
		if message.Method == "" {
			inflight.forget(jsonRPCIDKey(message.ID))
		}
	}
}

func (inflight *inflightRequests) forget(key string) {
	if key == "" {
		return
	}
	inflight.mu.Lock()
	defer inflight.mu.Unlock()
	if _, ok := inflight.pending[key]; !ok {
		return
	}
	delete(inflight.pending, key)
	close(inflight.changed)
	inflight.changed = make(chan struct{})
}

func (inflight *inflightRequests) idle() bool {
	inflight.mu.Lock()
	defer inflight.mu.Unlock()
	return len(inflight.pending) == 0
}

func (inflight *inflightRequests) waitIdle(limit time.Duration) {
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	for {
		inflight.mu.Lock()
		if len(inflight.pending) == 0 {
			inflight.mu.Unlock()
			return
		}
		changed := inflight.changed
		inflight.mu.Unlock()
		select {
		case <-changed:
		case <-deadline.C:
			return
		}
	}
}

type jsonRPCMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// decodeJSONRPCMessages는 한 줄의 단일 메시지나 batch를 읽는다. JSON이 아니면 아무것도
// 돌려주지 않는다. 그런 입력의 처리는 SDK가 정한다.
func decodeJSONRPCMessages(line []byte) []jsonRPCMessage {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return nil
	}
	if line[0] == '[' {
		var batch []jsonRPCMessage
		if json.Unmarshal(line, &batch) != nil {
			return nil
		}
		return batch
	}
	var message jsonRPCMessage
	if json.Unmarshal(line, &message) != nil {
		return nil
	}
	return []jsonRPCMessage{message}
}

// jsonRPCIDKey는 문자열 id와 숫자 id가 겹치지 않는 비교 키를 만든다. 숫자는
// go-sdk의 jsonrpc2.MakeID와 같이 float64로 읽어 int64로 바꾼다. SDK가 응답에 쓰는
// id가 그 값이라 `2.0`이나 `1e3` 요청도 응답과 짝이 맞는다.
func jsonRPCIDKey(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return ""
	}
	switch id := value.(type) {
	case string:
		return "s:" + id
	case float64:
		return "n:" + strconv.FormatInt(int64(id), 10)
	default:
		return ""
	}
}

// drainingReader는 한 줄씩 읽어 요청을 기록하고, 입력 EOF는 기록한 요청이 모두
// 응답된 뒤에 돌려준다.
type drainingReader struct {
	source   *bufio.Reader
	closer   io.Closer
	buffered []byte
	inflight *inflightRequests
}

func (reader *drainingReader) Read(p []byte) (int, error) {
	if len(reader.buffered) == 0 {
		line, err := reader.source.ReadBytes('\n')
		if len(line) == 0 {
			if errors.Is(err, io.EOF) {
				reader.inflight.waitIdle(mcpInputEOFDrainLimit)
			}
			return 0, err
		}
		reader.inflight.observeInbound(line)
		reader.buffered = line
	}
	n := copy(p, reader.buffered)
	reader.buffered = reader.buffered[n:]
	return n, nil
}

func (reader *drainingReader) Close() error {
	if reader.closer == nil {
		return nil
	}
	return reader.closer.Close()
}

// observingWriter는 서버가 쓴 바이트를 줄 단위로 모아 응답을 기록한다.
type observingWriter struct {
	target   io.Writer
	inflight *inflightRequests
	mu       sync.Mutex
	partial  []byte
}

func (writer *observingWriter) Write(p []byte) (int, error) {
	n, err := writer.target.Write(p)
	writer.mu.Lock()
	writer.partial = append(writer.partial, p[:n]...)
	for {
		end := bytes.IndexByte(writer.partial, '\n')
		if end < 0 {
			break
		}
		writer.inflight.observeOutbound(writer.partial[:end])
		writer.partial = writer.partial[end+1:]
	}
	writer.mu.Unlock()
	return n, err
}

func (writer *observingWriter) Close() error { return nil }
