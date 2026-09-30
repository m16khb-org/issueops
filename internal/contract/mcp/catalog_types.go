package mcp

// Tool describes a stable MCP tool schema fragment owned by the MCP adapter.
type Tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MimeType    string `json:"mimeType"`
}

// DispatchGroup names the handler group that owns a set of MCP tools.
// The MCP server uses this to route tool calls to the correct handler.
type DispatchGroup string

const (
	DispatchProject         DispatchGroup = "project"
	DispatchPolicyState     DispatchGroup = "policy_state"
	DispatchIssueOps        DispatchGroup = "issueops"
	DispatchLoop            DispatchGroup = "loop"
	DispatchGates           DispatchGroup = "gates"
	DispatchChannel         DispatchGroup = "channel"
	DispatchAssistantWorker DispatchGroup = "assistant_worker"
	DispatchSelfLoop        DispatchGroup = "self_loop"
)

// Catalog is the assembled transport catalog supplied by the composition root.
type Catalog struct {
	Tools     []map[string]any
	Resources []map[string]any
	Dispatch  map[string]DispatchGroup
}
