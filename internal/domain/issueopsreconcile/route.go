// Package issueopsreconcile owns the pure pending-intent recovery route.
package issueopsreconcile

type route string

// RouteNone and RouteRemotePR are the reconcile routes DecidePending can
// choose: nothing pending, or a pending remote pull request creation.
const (
	RouteNone        route = "none"
	RouteRemotePR    route = "remote_pr"
	routeUnsupported route = "unsupported"
)

// Decision is the routing outcome for a pending external intent: the Route to
// dispatch and the PreviewCode reported to the caller before confirmation.
type Decision struct {
	Route       route
	PreviewCode string
}

// DecidePending maps a pending external intent kind to its reconcile Decision.
// Without a pending intent it routes to RouteNone, "remote_pr_create" routes to
// RouteRemotePR, and any other kind is reported as unsupported.
func DecidePending(kind string, pending bool) Decision {
	if !pending {
		return Decision{Route: RouteNone, PreviewCode: "no_pending_external_intent"}
	}
	if kind == "remote_pr_create" {
		return Decision{Route: RouteRemotePR, PreviewCode: "remote_reconcile_required"}
	}
	return Decision{Route: routeUnsupported, PreviewCode: "unsupported_external_intent"}
}
