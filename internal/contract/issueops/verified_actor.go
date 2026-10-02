package issueops

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
