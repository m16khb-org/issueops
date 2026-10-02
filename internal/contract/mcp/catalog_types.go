package mcp

// Tool describes a stable MCP tool schema fragment owned by the MCP adapter.
type Tool struct {
	Name         string           `json:"name"`
	Description  string           `json:"description"`
	InputSchema  map[string]any   `json:"inputSchema"`
	OutputSchema map[string]any   `json:"outputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
}

// ToolAnnotations are the MCP behavior hints a tool advertises. A nil hint is
// omitted from tools/list so an unset hint never reads as a claim.
type ToolAnnotations struct {
	ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
	DestructiveHint *bool `json:"destructiveHint,omitempty"`
	IdempotentHint  *bool `json:"idempotentHint,omitempty"`
	OpenWorldHint   *bool `json:"openWorldHint,omitempty"`
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
