package mcpcli

import (
	"context"
	"io"
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	issueopscontract "issueops/internal/contract/issueops"
	"net"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestServeMCPStreamContextCancelsIdleSDKSession(t *testing.T) {
	server, client := net.Pipe()
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- ServeMCPStreamContextWithDependencies(ctx, server, server, io.Discard, MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Resources: resourceConfigForTest(), State: publicStateForTest()})
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("SDK MCP session ignored cancellation")
	}
}

func TestServeMCPStreamListsHarnessTools(t *testing.T) {
	session := startMCPTransportTestSession(t, "stdio", MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(), Resources: resourceConfigForTest(), State: publicStateForTest()})
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) == 0 {
		t.Fatalf("stream tool listing failed: tools=%#v err=%v", tools, err)
	}
	resource, err := session.ReadResource(context.Background(), &mcp.ReadResourceParams{URI: "issueops://commit-policy"})
	if err != nil || len(resource.Contents) == 0 {
		t.Fatalf("stream resource read failed: resource=%#v err=%v", resource, err)
	}
}

func TestServeMCPStreamCarriesPublicationReconcileWithoutInvokingOnHandshake(t *testing.T) {
	invoked := 0
	session := startMCPTransportTestSession(t, "stdio", MCPDependencies{Execution: testExecutionDeps(), Catalog: mcpcatalog.Build(),
		Publication: PublicationHandlers{Reconcile: func(context.Context, string, issueopscontract.ExecutionReconcileRequest) (issueopscontract.ExecutionReconcileResult, error) {
			invoked++
			return issueopscontract.ExecutionReconcileResult{}, nil
		}},
	})
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if invoked != 0 {
		t.Fatalf("stream handshake invoked publication reconcile handler: %d", invoked)
	}
	if len(tools.Tools) == 0 {
		t.Fatal("stream did not complete handshake and tool listing")
	}
}
