package port

// CleanupInvocationError means no operation result can be returned to the caller.
// It preserves the original observation or command-binding error for presentation.
type CleanupInvocationError struct{ Err error }

func (e *CleanupInvocationError) Error() string { return e.Err.Error() }
func (e *CleanupInvocationError) Unwrap() error { return e.Err }
