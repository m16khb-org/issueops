//go:build !darwin

package audit

func handoffDeliveryTrustedBases() []string {
	return nil
}
