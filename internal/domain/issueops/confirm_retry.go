package issueops

import "fmt"

// IdempotentConfirmRetry wraps the error of a confirmed provider write that
// may still have landed (a lost response) with the way back: re-run the same
// command, whose re-observation treats the settled state as success. Only
// writes that are idempotent by construction use it (reflect sections, issue
// and child closes); creates go through reconcile instead.
func IdempotentConfirmRetry(err error, settled string) error {
	return fmt.Errorf("%w; the provider may still have applied it: re-run this same command, which is idempotent (%s is success)", err, settled)
}
