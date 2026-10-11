package issueopscleanup

import "context"

// cleanupDrainer drains the execution lifetime at most once and keeps the
// latest lifetime so the caller can still close whatever Drain returned.
type cleanupDrainer struct {
	lifetime  CleanupLifetime
	attempted bool
	drained   bool
	err       error
}

func (d *cleanupDrainer) drain(ctx context.Context) error {
	if d.attempted {
		return d.err
	}
	d.attempted = true
	var next CleanupLifetime
	next, d.err = d.lifetime.Drain(ctx)
	d.lifetime = next
	d.drained = d.err == nil
	return d.err
}

func (d *cleanupDrainer) close() {
	if d.lifetime != nil {
		_ = d.lifetime.Close()
	}
}
