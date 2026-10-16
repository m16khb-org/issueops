// Package issueopsreconcile owns the pure pending-intent recovery route.
package issueopsreconcile

type Route string

const (
	RouteNone        Route = "none"
	RouteRemotePR    Route = "remote_pr"
	RouteUnsupported Route = "unsupported"
)

type Decision struct {
	Route       Route
	PreviewCode string
}

func DecidePending(kind string, pending bool) Decision {
	if !pending {
		return Decision{Route: RouteNone, PreviewCode: "no_pending_external_intent"}
	}
	if kind == "remote_pr_create" {
		return Decision{Route: RouteRemotePR, PreviewCode: "remote_reconcile_required"}
	}
	return Decision{Route: RouteUnsupported, PreviewCode: "unsupported_external_intent"}
}
