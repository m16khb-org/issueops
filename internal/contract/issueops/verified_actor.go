package issueops

// The VerifiedBy* values name how a VerifiedActor was authenticated: by native
// process ancestry, or by a capability token.
const (
	VerifiedByNativeAncestry = "native_ancestry"
	VerifiedByCapability     = "capability"
)

// VerifiedActor is an internal verifier result, not a client-supplied assertion.
// Capability verification does not manufacture observed process ancestry.
type VerifiedActor struct {
	Identity NativeActor `json:"-"`
	Method   string      `json:"-"`
}
