package mcpcli

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

// host나 셸 파이프라인이 요청을 보낸 직후 stdin을 닫아도, 이미 받은 요청의 응답은
// 모두 돌려준 뒤 세션을 닫아야 한다. 예전 daemon proxy가 지키던 동작이다(8a729a33).
func TestServeMCPStreamAnswersEveryRequestReadBeforeInputEOF(t *testing.T) {
	input := strings.Join([]string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"eof","version":"0"}}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`,
		`{"jsonrpc":"2.0","id":"three","method":"resources/read","params":{"uri":"issueops://commit-policy"}}`,
	}, "\n") + "\n"
	var output bytes.Buffer
	_ = ServeMCPStreamWithDependencies(strings.NewReader(input), &output, io.Discard, MCPDependencies{})
	answered := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n") {
		if line == "" {
			continue
		}
		var message struct {
			ID     json.RawMessage `json:"id"`
			Result json.RawMessage `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			t.Fatalf("server wrote a non-JSON line %q: %v", line, err)
		}
		if len(message.Result) > 0 {
			answered[string(message.ID)] = true
		}
	}
	for _, id := range []string{`1`, `2`, `"three"`} {
		if !answered[id] {
			t.Fatalf("request %s was read before input EOF but never answered; output:\n%s", id, output.String())
		}
	}
}

// 취소된 요청은 응답이 오지 않을 수 있으므로 EOF 뒤 대기 목록에서 빠져야 한다.
func TestInflightRequestsForgetCancelledRequests(t *testing.T) {
	inflight := newInflightRequests()
	inflight.observeInbound([]byte(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{}}`))
	if inflight.idle() {
		t.Fatal("a request read from the host must be in flight")
	}
	inflight.observeInbound([]byte(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7}}`))
	if !inflight.idle() {
		t.Fatal("a cancelled request must not keep the session open")
	}
	inflight.observeInbound([]byte(`[{"jsonrpc":"2.0","id":"a","method":"ping"},{"jsonrpc":"2.0","id":8,"method":"ping"}]`))
	inflight.observeOutbound([]byte(`{"jsonrpc":"2.0","id":"a","result":{}}`))
	if inflight.idle() {
		t.Fatal("request 8 is still unanswered")
	}
	inflight.observeOutbound([]byte(`{"jsonrpc":"2.0","id":8,"error":{"code":-1,"message":"x"}}`))
	if !inflight.idle() {
		t.Fatal("every request in a batch was answered")
	}
}

// go-sdk는 숫자 id를 float64로 읽은 뒤 int64로 바꿔 응답한다. 요청 키도 같은 규칙을
// 따라야 `2.0`, `1e3` 같은 id의 응답이 대기 목록을 비운다.
func TestInflightRequestsMatchSDKNumericIDs(t *testing.T) {
	inflight := newInflightRequests()
	inflight.observeInbound([]byte(`[{"jsonrpc":"2.0","id":2.0,"method":"ping"},{"jsonrpc":"2.0","id":1e3,"method":"ping"}]`))
	inflight.observeOutbound([]byte(`{"jsonrpc":"2.0","id":2,"result":{}}`))
	inflight.observeOutbound([]byte(`{"jsonrpc":"2.0","id":1000,"result":{}}`))
	if !inflight.idle() {
		t.Fatal("responses with the SDK's int64 ids must clear the float-form request ids")
	}
}

// 응답이 짝을 찾지 못하는 경우에도 입력 EOF 뒤 대기는 상한 안에서 끝나야 한다.
func TestInflightRequestsWaitIdleStopsAtTheLimit(t *testing.T) {
	inflight := newInflightRequests()
	inflight.observeInbound([]byte(`{"jsonrpc":"2.0","id":"never","method":"ping"}`))
	done := make(chan struct{})
	go func() {
		inflight.waitIdle(20 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waitIdle must return once the drain limit passes")
	}
}
