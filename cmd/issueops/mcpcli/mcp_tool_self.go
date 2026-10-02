package mcpcli

import (
	"context"
	"time"

	"issueops/cmd/issueops/mcpcli/argmap"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func handleSelfLoopMCPToolCall(ctx context.Context, call MCPToolCall, deps MCPDependencies) MCPToolOutcome {
	switch call.Name {
	case "self_augment":
		result, err := augmentapp.PlanAndSave(augmentcontract.SelfAugmentPlanRequest{
			Cycles:      argmap.Int(call.Arguments, "cycles", 1),
			TargetScore: argmap.Float(call.Arguments, "target_score", 95),
		}, argmap.Bool(call.Arguments, "save_state"), argmap.StringDefault(call.Arguments, "state_key", "self-augment-latest"), augmentapp.PlanAndSaveDeps{Plan: deps.SelfPlanning.Plan, Save: func(result *augmentcontract.SelfAugmentPlanResult, key string) error {
			return deps.SelfState.SavePlan(ctx, result, key)
		}})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "Self-augmentation plan save failed", result))
		}
		return mcpToolPayload(result)
	case "self_augment_lesson":
		result, err := deps.SelfPlanning.SaveLesson(ctx, augmentcontract.SelfAugmentLessonRequest{
			CandidateID: argmap.String(call.Arguments, "candidate_id"),
			Lesson:      argmap.String(call.Arguments, "lesson"),
			NextAction:  argmap.String(call.Arguments, "next_action"),
			Source:      argmap.StringDefault(call.Arguments, "source", "self-augment"),
			Severity:    argmap.StringDefault(call.Arguments, "severity", "info"),
			StateKey:    argmap.String(call.Arguments, "state_key"),
		})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-augmentation lesson save failed", result))
		}
		return mcpToolPayload(result)
	case "self_verify":
		if value, present := call.Arguments["base_ref"]; present {
			if ref, ok := value.(string); !ok || ref == "" {
				return mcpToolFailure(newProtocolError(-32602, "base_ref must name a commit", nil))
			}
		}
		seed := argmap.Int64(call.Arguments, "seed", time.Now().Unix())
		targetScore := argmap.Float(call.Arguments, "target_score", 95)
		result, err := verifyapp.Execute(verifyapp.ExecuteRequest{
			Loop:      verifyapp.LoopRequest{BaseSeed: seed, TargetScore: targetScore, BaseRef: argmap.String(call.Arguments, "base_ref")},
			SaveState: argmap.Bool(call.Arguments, "save_state"),
			StateKey:  argmap.StringDefault(call.Arguments, "state_key", "self-verify-latest"),
		}, verifyapp.ExecuteDeps{
			Verify: deps.SelfVerify,
			SaveSummary: func(result *augmentcontract.SelfAugmentResult, key string) error {
				return deps.SelfState.SaveSummary(ctx, result, key)
			},
		})
		if err != nil && !isSelfVerificationGateError(err) {
			return mcpToolFailure(newProtocolError(-32000, "Self-verification failed", result))
		}
		return mcpToolPayload(result)
	case "self_verify_candidates":
		result, err := verifyapp.ExportAndSaveCandidates(argmap.Bool(call.Arguments, "save_state"), argmap.StringDefault(call.Arguments, "state_key", "self-verify-candidates-latest"), verifyapp.ExportAndSaveCandidatesDeps{Export: deps.SelfPlanning.ExportCandidates, Save: func(result *augmentcontract.SelfVerificationCandidateExportResult, key string) error {
			return deps.SelfPlanning.SaveCandidates(ctx, result, key)
		}})
		if err != nil {
			return mcpToolFailure(newProtocolError(-32000, "Self-verify candidate export save failed", result))
		}
		return mcpToolPayload(result)
	case "self_verify_history", "self_augment_history":
		result, err := deps.SelfHistory.History(
			ctx,
			argmap.StringDefault(call.Arguments, "prefix", "self-verify"),
			argmap.Int(call.Arguments, "limit", 20),
			augmentcontract.SelfAugmentHistoryRetentionOptions{
				Limit:          argmap.Int(call.Arguments, "retention_limit", 0),
				PruneRequested: argmap.Bool(call.Arguments, "prune_retention"),
				Confirm:        argmap.Bool(call.Arguments, "confirm"),
			},
		)
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-verify history failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "self_verify_compare", "self_augment_compare":
		result, err := deps.SelfHistory.Compare(
			argmap.String(call.Arguments, "baseline_key"),
			argmap.String(call.Arguments, "candidate_key"),
			argmap.Float(call.Arguments, "max_elapsed_regression_pct", 20),
		)
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-verify compare failed", err.Error()))
		}
		return mcpToolPayload(result)
	case "self_verify_promote", "self_augment_promote":
		result, err := deps.SelfState.Promote(
			ctx,
			argmap.String(call.Arguments, "from_key"),
			argmap.String(call.Arguments, "baseline_key"),
			argmap.Bool(call.Arguments, "confirm"),
			argmap.Bool(call.Arguments, "allow_failed_source"),
		)
		if err != nil {
			return mcpToolFailure(newProtocolError(-32602, "Self-verify promote failed", err.Error()))
		}
		return mcpToolPayload(result)
	default:
		return MCPToolOutcome{}
	}
}
