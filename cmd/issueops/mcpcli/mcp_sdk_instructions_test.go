package mcpcli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func initializeInstructions(t *testing.T, transport serverTransport) string {
	t.Helper()
	deps := testTransportServices()
	deps.Catalog = testMCPCatalog()
	deps.Resources = resourceConfigForTest()
	server := newSDKServer(deps, sdkServerOptions(), transport, nil)
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer serverSession.Close()
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()
	return clientSession.InitializeResult().Instructions
}

func TestInitializeInstructionsFollowTransport(t *testing.T) {
	stdio := initializeInstructions(t, transportStdio)
	if !strings.Contains(stdio, "in-process") {
		t.Fatalf("stdio instructions must keep the in-process wording: %q", stdio)
	}

	http := initializeInstructions(t, transportHTTP)
	for _, want := range []string{"shared local service", "authority_file", "issueops mcp authorize"} {
		if !strings.Contains(http, want) {
			t.Fatalf("HTTP instructions missing %q: %q", want, http)
		}
	}
	if strings.Contains(http, "in-process") {
		t.Fatalf("HTTP instructions must not claim an in-process host session: %q", http)
	}
}
