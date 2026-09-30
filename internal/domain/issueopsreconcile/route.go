// Package issueopsreconcile owns the pure pending-intent recovery route.
package issueopsreconcile

import "strings"

type Route string

const (
	RouteNone        Route = "none"
	RouteRemotePR    Route = "remote_pr"
	RouteOrca        Route = "orca"
	RouteUnsupported Route = "unsupported"
)

type Decision struct {
	Route             Route
	PreviewCode       string
	SkipMutationGuard bool
}

func DecidePending(mode, kind string, pending bool) Decision {
	if !pending {
		return Decision{Route: RouteNone, PreviewCode: "no_pending_external_intent"}
	}
	normalized := strings.TrimSpace(kind)
	decision := Decision{Route: RouteUnsupported, PreviewCode: "unsupported_external_intent"}
	switch {
	case normalized == "remote_pr_create":
		decision.PreviewCode = "remote_reconcile_required"
	case IsOrcaIntentKind(normalized):
		decision.PreviewCode = "orca_reconcile_required"
		decision.SkipMutationGuard = mode == "orca"
	}
	switch {
	case kind == "remote_pr_create":
		decision.Route = RouteRemotePR
	case exactOrcaIntentKind(kind):
		decision.Route = RouteOrca
	}
	return decision
}

func IsOrcaIntentKind(kind string) bool {
	return exactOrcaIntentKind(strings.TrimSpace(kind))
}

func exactOrcaIntentKind(kind string) bool {
	switch kind {
	case "worktree_create", "owner_launch", "dispatch":
		return true
	default:
		return false
	}
}
