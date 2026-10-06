package mcp

import contract "issueops/internal/contract/mcp"

// catalogSection binds one catalog function to its dispatch handler group and
// records whether its tools are advertised in the tools/list response.
type catalogSection struct {
	group      contract.DispatchGroup
	advertised bool
	tools      func() []contract.Tool
}

// catalogSections is the single ordered source of truth for the MCP tool
// catalog. Both the advertised tools/list (AdvertisedTools) and the
// name->handler routing table (DispatchMap) derive from this slice, so adding a
// tool means editing exactly one catalog function referenced here. The
// advertised order matches the stable mcp_tools.golden.json snapshot.
func catalogSections() []catalogSection {
	return []catalogSection{
		{contract.DispatchProject, true, contract.CoreProjectTools},
		{contract.DispatchPolicyState, true, contract.CommandPolicyTools},
		{contract.DispatchPolicyState, true, contract.StateTools},
		{contract.DispatchIssueOps, true, contract.IssueOpsBasicTools},
		{contract.DispatchLoop, true, contract.LoopTools},
		{contract.DispatchGates, true, contract.GatesTools},
		{contract.DispatchChannel, true, contract.ChannelTools},
		{contract.DispatchSelfLoop, true, contract.SelfLoopAdvertisedTools},
		{contract.DispatchAssistantWorker, true, contract.AdapterOwnedTools},
		{contract.DispatchPolicyState, true, contract.CommandPolicyAuditTools},
		{contract.DispatchAssistantWorker, true, contract.LocalAssistantTools},
		{contract.DispatchSelfLoop, false, contract.SelfLoopAliasTools},
	}
}

// AdvertisedTools returns every MCP tool advertised in tools/list, in the
// stable order pinned by mcp_tools.golden.json.
func AdvertisedTools() []contract.Tool {
	var out []contract.Tool
	for _, s := range catalogSections() {
		if s.advertised {
			out = append(out, s.tools()...)
		}
	}
	return out
}

// DispatchMap returns a map from every MCP tool name to its handler group.
// It derives from catalogSections so routing can never drift from the catalog:
// adding a tool to a section makes it both routable and (if advertised) listed.
func DispatchMap() map[string]contract.DispatchGroup {
	out := make(map[string]contract.DispatchGroup)
	for _, s := range catalogSections() {
		for _, t := range s.tools() {
			out[t.Name] = s.group
		}
	}
	return out
}

func ToolMaps(tools []contract.Tool) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		toolMap := map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.InputSchema,
		}
		if tool.OutputSchema != nil {
			toolMap["outputSchema"] = tool.OutputSchema
		}
		if tool.Annotations != nil {
			toolMap["annotations"] = tool.Annotations
		}
		out = append(out, toolMap)
	}
	return out
}

func ResourceMaps(resources []contract.Resource) []map[string]any {
	out := make([]map[string]any, 0, len(resources))
	for _, resource := range resources {
		out = append(out, map[string]any{
			"uri":         resource.URI,
			"name":        resource.Name,
			"description": resource.Description,
			"mimeType":    resource.MimeType,
		})
	}
	return out
}

func Build() contract.Catalog {
	return contract.Catalog{Tools: withAuthorityFields(ToolMaps(AdvertisedTools())), Resources: ResourceMaps(contract.Resources()), Dispatch: DispatchMap()}
}
