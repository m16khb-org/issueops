package mcpcli

import (
	"context"
	"errors"
	"strings"

	"issueops/cmd/issueops/mcpcli/argmap"
	authoritycontract "issueops/internal/contract/authority"
	authoritydomain "issueops/internal/domain/authority"
)

type serverTransport uint8

const (
	transportStdio serverTransport = iota
	transportHTTP
)

func dispatchMCPToolCall(ctx context.Context, call MCPToolCall, deps MCPDependencies, transport serverTransport) MCPToolOutcome {
	spec := mcpToolAuthorities[call.Name]
	if !requiresCapability(spec, call.Arguments, transport) {
		return resolveHandlerGroup(deps, call.Name)(ctx, call)
	}
	requestCtx, requestDeps, args, err := authorizeMCPRequest(ctx, call, deps, spec, transport)
	if err != nil {
		return mcpToolErrorPayload(authorityErrorPayload(err))
	}
	call.Arguments = args
	return resolveHandlerGroup(requestDeps, call.Name)(requestCtx, call)
}

// requiresCapability: shared HTTP demands a capability for every workspace
// call; stdio keeps native ancestry unless the caller supplies a capability.
func requiresCapability(spec toolAuthority, args map[string]any, transport serverTransport) bool {
	if spec.scope != toolScopeWorkspace {
		return false
	}
	if transport == transportStdio {
		return strings.TrimSpace(argmap.String(args, argAuthorityFile)) != ""
	}
	if !spec.optional {
		return true
	}
	for _, field := range append([]string{argAuthorityFile, argWorkspaceRoot}, spec.rootArgs...) {
		if strings.TrimSpace(argmap.String(args, field)) != "" {
			return true
		}
	}
	return false
}

func authorizeMCPRequest(ctx context.Context, call MCPToolCall, deps MCPDependencies, spec toolAuthority, transport serverTransport) (context.Context, MCPDependencies, map[string]any, error) {
	if transport == transportHTTP {
		for _, field := range actorArguments {
			if _, present := call.Arguments[field]; present {
				return nil, MCPDependencies{}, nil, authorityInvalid("actor identity fields cannot accompany a capability on shared HTTP")
			}
		}
	}
	file := strings.TrimSpace(argmap.String(call.Arguments, argAuthorityFile))
	if file == "" {
		return nil, MCPDependencies{}, nil, authorityRequired("authority_file is required for workspace tools")
	}
	if deps.RequestScope == nil || deps.Credentials == nil || deps.BindAuthority == nil || deps.ForRequest == nil {
		return nil, MCPDependencies{}, nil, authorityInvalid("request authority is not configured")
	}
	scope, err := deps.RequestScope(ctx, call.Name, call.Arguments)
	if err != nil {
		return nil, MCPDependencies{}, nil, err
	}
	key, token, err := deps.Credentials.Read(ctx, file)
	if err != nil {
		return nil, MCPDependencies{}, nil, authorityInvalid("authority_file is not a readable managed credential")
	}
	bound, verified, err := deps.BindAuthority(ctx, authoritycontract.Use{
		Key: key, Token: token, WorkspaceRoot: scope.WorkspaceRoot, CWD: scope.CWD,
		Tool: call.Name, Action: argmap.String(call.Arguments, "action"),
	})
	if err != nil {
		return nil, MCPDependencies{}, nil, err
	}
	requestCtx, requestDeps, err := deps.ForRequest(bound, scope, verified)
	if err != nil {
		return nil, MCPDependencies{}, nil, err
	}
	requestDeps.Caller = &verified
	return requestCtx, requestDeps, scopedArguments(call.Arguments, spec, scope), nil
}

func authorityErrorPayload(err error) map[string]any {
	payload := map[string]any{"ok": false, "error": err.Error()}
	if typed, ok := errors.AsType[*requestAuthorityError](err); ok {
		payload["error_code"] = typed.code
	} else if typed, ok := errors.AsType[*authoritydomain.Error](err); ok {
		payload["error_code"] = typed.Code
	} else if errors.Is(err, authoritycontract.ErrInvalidState) {
		payload["error_code"] = authoritycontract.CodeInvalid
	}
	return payload
}
