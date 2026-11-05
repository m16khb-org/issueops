package mcpcli

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	channelapp "issueops/internal/application/channel"
	channelcontract "issueops/internal/contract/channel"
)

// waitingChannelEffects makes channel_recv(wait) block inside Effects.Wait so a
// test can hold a tool call in flight. entered closes when the call is waiting.
// honorContext selects whether the wait ends on ctx.Done or only on release.
type waitingChannelEffects struct {
	entered      chan struct{}
	release      chan struct{}
	honorContext bool
	once         sync.Once
}

func newWaitingChannelEffects(honorContext bool) *waitingChannelEffects {
	return &waitingChannelEffects{entered: make(chan struct{}), release: make(chan struct{}), honorContext: honorContext}
}

func (*waitingChannelEffects) Open() (channelapp.Writer, error) { return nil, errors.New("unused") }
func (*waitingChannelEffects) NewID() string                    { return "id" }
func (*waitingChannelEffects) Now() time.Time                   { return time.Now() }
func (*waitingChannelEffects) MessagesAfter(context.Context, string) ([]channelcontract.Message, error) {
	return nil, nil
}

func (e *waitingChannelEffects) Wait(ctx context.Context, _ time.Duration) error {
	e.once.Do(func() { close(e.entered) })
	if e.honorContext {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-e.release:
			return nil
		}
	}
	<-e.release
	return nil
}

const stdioHandshakeAndWaitingCall = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"shutdown","version":"0"}}}` + "\n" +
	`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
	`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"channel_recv","arguments":{"channel":"wait","wait":true,"timeout_seconds":300}}}` + "\n"

// startShutdownServe serves one stdio session over pipes with a channel_recv
// call in flight. It returns the stdin writer and the channel that receives
// serveMCPStream's result.
func startShutdownServe(t *testing.T, ctx context.Context, effects *waitingChannelEffects, drain time.Duration) (*io.PipeWriter, <-chan error) {
	t.Helper()
	stdin, stdinWriter := io.Pipe()
	t.Cleanup(func() { _ = stdinWriter.Close() })
	deps := MCPDependencies{Catalog: mcpcatalog.Build(), Channel: channelapp.Service{Effects: effects}}
	done := make(chan error, 1)
	go func() { done <- serveMCPStream(ctx, stdin, &syncBuffer{}, io.Discard, deps, drain) }()
	if _, err := io.WriteString(stdinWriter, stdioHandshakeAndWaitingCall); err != nil {
		t.Fatal(err)
	}
	select {
	case <-effects.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("channel_recv never started waiting")
	}
	return stdinWriter, done
}

// A shutdown signal while stdin stays open must cancel the request that is
// still waiting once the drain limit passes, and end with a nil result.
func TestServeMCPStreamCancelsInFlightRequestOnShutdown(t *testing.T) {
	effects := newWaitingChannelEffects(true)
	ctx, cancel := context.WithCancel(t.Context())
	_, done := startShutdownServe(t, ctx, effects, 100*time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve after shutdown = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after the shutdown signal")
	}
}

// With stdin already at EOF the reader waits for the unanswered request. A
// shutdown must end that wait, and a handler that never looks at its context
// must not keep the process alive past the drain and stuck limits.
func TestServeMCPStreamStopsDrainOnContextCancel(t *testing.T) {
	effects := newWaitingChannelEffects(false)
	t.Cleanup(func() { close(effects.release) })
	ctx, cancel := context.WithCancel(t.Context())
	stdinWriter, done := startShutdownServe(t, ctx, effects, 100*time.Millisecond)
	if err := stdinWriter.Close(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serve after shutdown = %v, want nil", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("serve did not return after the shutdown signal")
	}
}

func TestInflightRequestsWaitIdleStopsOnContextCancel(t *testing.T) {
	inflight := newInflightRequests()
	inflight.observeInbound([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call"}`))
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		inflight.waitIdle(ctx, time.Minute)
		close(done)
	}()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("waitIdle ignored context cancellation")
	}
}
