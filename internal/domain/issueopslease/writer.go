package issueopslease

// LeaseHoldsWriter treats unknown lease statuses as an active writer.
func LeaseHoldsWriter(status string) bool {
	return status != "claimable" && status != "released"
}
