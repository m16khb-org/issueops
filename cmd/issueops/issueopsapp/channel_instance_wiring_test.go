package issueopsapp

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"issueops/cmd/issueops/mcpcli"
	model "issueops/internal/contract/channel"
)

func TestChannelCLIAndMCPKeepCapturedStores(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	ambient := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", firstRoot)
	firstCLI := channelDependencies()
	firstMCP := issueOpsMCPDependencies()
	t.Setenv("ISSUEOPS_STATE_DIR", secondRoot)
	secondCLI := channelDependencies()
	secondMCP := issueOpsMCPDependencies()
	t.Setenv("ISSUEOPS_STATE_DIR", ambient)
	var wg sync.WaitGroup
	for i, deps := range []mcpcli.MCPDependencies{firstMCP, secondMCP} {
		wg.Go(func() {
			for j := range 4 {
				raw, _ := json.Marshal(mcpcli.MCPToolCall{Name: "channel_send", Arguments: map[string]any{"channel": "same", "from": "test", "body": fmt.Sprintf("store-%d", i)}})
				_, err := mcpcli.HandleToolCallWithDependencies(raw, deps)
				if err != nil {
					t.Errorf("send %d: %v", j, err)
				}
			}
		})
	}
	wg.Wait()
	explicit, err := newChannelService(firstRoot).Recv(model.RecvRequest{Channel: "same"})
	if err != nil || len(explicit.Messages) != 4 {
		t.Fatalf("explicit root ignored: %+v %v", explicit, err)
	}
	for i, receive := range []func(model.RecvRequest) (model.RecvResult, error){firstCLI.Recv, secondCLI.Recv} {
		result, err := receive(model.RecvRequest{Channel: "same"})
		if err != nil || len(result.Messages) != 4 {
			t.Fatalf("store=%d result=%+v err=%v", i, result, err)
		}
		for _, message := range result.Messages {
			if message.Body != fmt.Sprintf("store-%d", i) {
				t.Fatal("MCP servers shared channel state")
			}
		}
	}
	result, err := newChannelService(ambient).Recv(model.RecvRequest{Channel: "same"})
	if err != nil || len(result.Messages) != 0 {
		t.Fatalf("ambient state was touched: %+v %v", result, err)
	}
}
