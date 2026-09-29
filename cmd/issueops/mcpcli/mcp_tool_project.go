package mcpcli

import (
	"issueops/cmd/issueops/apidoc"
	"issueops/cmd/issueops/mcpcli/argmap"
	projectbootstrapcontract "issueops/internal/contract/projectbootstrap"
	projectdocscontract "issueops/internal/contract/projectdocs"
)

func handleProjectMCPToolCall(call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	switch call.Name {
	case "harness_inspect":
		return mcpToolPayload(deps.Inspect(argmap.String(call.Arguments, "repo")))
	case "atomic_commit_preflight":
		return mcpToolPayload(deps.Preflight.Check(deps.resolveTarget(argmap.String(call.Arguments, "path")), deps.Resources.IssueOpsRoot))
	case "commit_policy":
		text, err := deps.Resources.ReadHarnessFile(".issueops", "COMMIT_POLICY.md")
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "Cannot read commit policy", err.Error()))
		}
		return mcpToolDirect(TextResult(text))
	case "skill_manifest":
		return mcpToolPayload(deps.Skills(deps.Resources.IssueOpsRoot, deps.Resources.SkillName))
	case "docs_index":
		return mcpToolPayload(deps.Resources.DocsIndex(deps.Resources.IssueOpsRoot, deps.Resources.Version))
	case "project_docs_route":
		result, err := deps.ProjectDocs.Route(argmap.String(call.Arguments, "repo"), argmap.StringDefault(call.Arguments, "task", "general"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs route failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_bootstrap_plan":
		result, err := deps.ProjectBootstrap.Run(projectbootstrapcontract.ProjectDocsBootstrapRequest{RepoRoot: argmap.String(call.Arguments, "repo"), Write: false})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs bootstrap plan failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_read":
		result, err := deps.ProjectDocs.Read(argmap.String(call.Arguments, "repo"), argmap.String(call.Arguments, "rel_path"))
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs read failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_revise":
		result, err := deps.ProjectDocs.Revise(projectdocscontract.ProjectDocsReviseRequest{
			RepoRoot:       argmap.String(call.Arguments, "repo"),
			RelPath:        argmap.String(call.Arguments, "rel_path"),
			Content:        argmap.String(call.Arguments, "content"),
			ExpectedSHA256: argmap.String(call.Arguments, "expected_sha256"),
			Summary:        argmap.String(call.Arguments, "summary"),
			Evidence:       argmap.StringSlice(call.Arguments, "evidence"),
			Confirm:        argmap.Bool(call.Arguments, "confirm"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs revise failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "project_docs_append":
		result, err := deps.ProjectDocs.Append(projectdocscontract.ProjectDocsAppendRequest{
			RepoRoot:     argmap.String(call.Arguments, "repo"),
			Kind:         argmap.String(call.Arguments, "kind"),
			Title:        argmap.String(call.Arguments, "title"),
			Summary:      argmap.String(call.Arguments, "summary"),
			Context:      argmap.String(call.Arguments, "context"),
			Resolution:   argmap.String(call.Arguments, "resolution"),
			Decision:     argmap.String(call.Arguments, "decision"),
			Evidence:     argmap.StringSlice(call.Arguments, "evidence"),
			Alternatives: argmap.StringSlice(call.Arguments, "alternatives"),
			Consequences: argmap.String(call.Arguments, "consequences"),
			Source:       argmap.StringDefault(call.Arguments, "source", "mcp"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Project docs append failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "api_doc_review":
		result, err := apidoc.RunReviewWithOptions(apidoc.ReviewOptions{
			Repo:       deps.resolveTarget(argmap.String(call.Arguments, "repo")),
			Files:      argmap.StringSlice(call.Arguments, "files"),
			All:        argmap.Bool(call.Arguments, "all"),
			DiffFile:   argmap.String(call.Arguments, "diff_file"),
			PromptFile: argmap.String(call.Arguments, "prompt_file"),
			ResultFile: argmap.String(call.Arguments, "result_file"),
			JSON:       true,
		})
		if err != nil && !apidoc.IsReviewGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "API doc review failed", result))
		}
		return mcpToolPayload(result)
	case "api_doc_static_check":
		result, err := apidoc.RunStaticCheckWithOptions(apidoc.StaticOptions{
			Repo:  deps.resolveTarget(argmap.String(call.Arguments, "repo")),
			Files: argmap.StringSlice(call.Arguments, "files"),
			All:   argmap.Bool(call.Arguments, "all"),
			JSON:  true,
		})
		if err != nil && !apidoc.IsStaticGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "API doc static check failed", result))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
