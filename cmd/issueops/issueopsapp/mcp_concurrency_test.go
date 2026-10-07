package issueopsapp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestConcurrentRootCommandsDoNotRewireMCPDependencies(t *testing.T) {
	const commandCount = 80
	wireDependencies()
	ready := make(chan struct{}, commandCount)
	release := make(chan struct{})
	var group sync.WaitGroup
	group.Add(commandCount)

	for range commandCount {
		go func() {
			defer group.Done()
			ready <- struct{}{}
			<-release
			if code := RunRootCommand([]string{"--version"}); code != 0 {
				t.Errorf("version exit code = %d", code)
			}
		}()
	}
	for range commandCount {
		<-ready
	}
	close(release)
	group.Wait()
}

func TestConcurrentMCPStreamsShareImmutableConfiguration(t *testing.T) {
	const sessionCount = 80

	wireDependencies()
	// Concurrent sessions share the CPU, so each session's latency grows with
	// host and package-level load. Bound the whole run by the test binary's own
	// deadline instead of per-session wall-clock timers.
	ctx := concurrencyTestContext(t)
	ready := make(chan struct{}, sessionCount)
	release := make(chan struct{})
	results := make(chan mcpStreamResult, sessionCount)

	for range sessionCount {
		go func() {
			ready <- struct{}{}
			<-release
			results <- exerciseMCPStream(ctx)
		}()
	}

	for range sessionCount {
		select {
		case <-ready:
		case <-ctx.Done():
			t.Fatalf("MCP sessions did not reach the fenced start: %v", ctx.Err())
		}
	}
	close(release)

	var expected []string
	for range sessionCount {
		select {
		case result := <-results:
			if result.err != nil {
				t.Fatal(result.err)
			}
			if expected == nil {
				expected = result.tools
				continue
			}
			if !slices.Equal(result.tools, expected) {
				t.Fatalf("MCP tool catalogs differ: got=%v want=%v", result.tools, expected)
			}
		case <-ctx.Done():
			t.Fatalf("MCP sessions did not complete: %v", ctx.Err())
		}
	}
}

// concurrencyTestContext ends shortly before the test binary's deadline so a
// stuck session fails with a test error rather than the binary's panic dump.
func concurrencyTestContext(t *testing.T) context.Context {
	t.Helper()
	deadline, ok := t.Deadline()
	if !ok {
		return t.Context()
	}
	const teardownMargin = 30 * time.Second
	ctx, cancel := context.WithDeadline(t.Context(), deadline.Add(-teardownMargin))
	t.Cleanup(cancel)
	return ctx
}

type mcpStreamResult struct {
	tools []string
	err   error
}

func exerciseMCPStream(parent context.Context) mcpStreamResult {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- serveMCPStreamContext(ctx, serverConn, serverConn, io.Discard)
	}()

	client := mcp.NewClient(&mcp.Implementation{Name: "concurrent-harness-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	if err != nil {
		_ = clientConn.Close()
		_ = serverConn.Close()
		return mcpStreamResult{err: err}
	}
	var names []string
	tools, listErr := session.ListTools(ctx, nil)
	if listErr == nil {
		names = make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
		}
		sort.Strings(names)
	}
	closeErr := session.Close()
	_ = clientConn.Close()
	var serverErr error
	select {
	case serverErr = <-serverDone:
	case <-ctx.Done():
		return mcpStreamResult{err: fmt.Errorf("MCP server teardown: %w", ctx.Err())}
	}
	if serverErr != nil && !errors.Is(serverErr, context.Canceled) && !errors.Is(serverErr, net.ErrClosed) {
		return mcpStreamResult{err: serverErr}
	}
	if listErr != nil {
		return mcpStreamResult{err: listErr}
	}
	if closeErr != nil {
		return mcpStreamResult{err: closeErr}
	}
	if len(names) == 0 {
		return mcpStreamResult{err: errors.New("MCP tool catalog is empty")}
	}
	return mcpStreamResult{tools: names}
}
