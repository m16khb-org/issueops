package mcpcli

import (
	"context"
	"fmt"
	loopapp "issueops/internal/application/looprun"

	"issueops/cmd/issueops/mcpcli/argmap"
	loopruncontract "issueops/internal/contract/looprun"
)

var loopMCPHandlers = map[string]func(context.Context, map[string]any, loopapp.Service) MCPToolOutcome{
	"loop_start":          handleMCPLoopStart,
	"loop_record_attempt": handleMCPLoopRecordAttempt,
	"loop_status":         handleMCPLoopStatus,
	"loop_stop":           handleMCPLoopStop,
}

func handleLoopMCPToolCall(ctx context.Context, call MCPToolCall, service loopapp.Service) MCPToolOutcome {
	handler, ok := loopMCPHandlers[call.Name]
	if !ok {
		return MCPToolOutcome{}
	}
	return handler(ctx, call.Arguments, service)
}

func loopMCPOutcome(payload any, err error, message string) MCPToolOutcome {
	if err != nil {
		return mcpToolErrorPayload(map[string]any{
			"ok":    false,
			"error": fmt.Sprintf("%s: %s", message, err.Error()),
		})
	}
	return mcpToolPayload(payload)
}

func handleMCPLoopStart(ctx context.Context, args map[string]any, service loopapp.Service) MCPToolOutcome {
	result, err := service.Start(ctx, loopruncontract.StartLoopRequest{
		Repo:        argmap.String(args, "repo"),
		Name:        argmap.String(args, "name"),
		Goal:        argmap.String(args, "goal"),
		VerifyArgv:  argmap.StringSlice(args, "verify_argv"),
		MaxAttempts: argmap.Int(args, "max_attempts", 0),
	})
	return loopMCPOutcome(result, err, "Loop start failed")
}

func handleMCPLoopRecordAttempt(ctx context.Context, args map[string]any, service loopapp.Service) MCPToolOutcome {
	result, err := service.RecordAttempt(ctx, argmap.String(args, "id"), loopruncontract.RecordAttemptRequest{
		Verdict:  argmap.String(args, "verdict"),
		Evidence: argmap.StringSlice(args, "evidence"),
	})
	return loopMCPOutcome(result, err, "Loop record-attempt failed")
}

func handleMCPLoopStatus(_ context.Context, args map[string]any, service loopapp.Service) MCPToolOutcome {
	result, err := service.Status(argmap.String(args, "id"))
	return loopMCPOutcome(result, err, "Loop status failed")
}

func handleMCPLoopStop(ctx context.Context, args map[string]any, service loopapp.Service) MCPToolOutcome {
	result, err := service.Stop(ctx, argmap.String(args, "id"), argmap.Bool(args, "success"), argmap.String(args, "reason"))
	return loopMCPOutcome(result, err, "Loop stop failed")
}
