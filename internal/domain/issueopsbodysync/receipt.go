package issueopsbodysync

import "fmt"

func ValidateReadback(applied bool, observed, intended string) error {
	if !applied {
		return fmt.Errorf("provider did not report the body replacement as applied")
	}
	if observed != intended {
		return fmt.Errorf("remote body readback does not match what was written (readback %s, intended %s)", observed, intended)
	}
	return nil
}

func ValidateChildHierarchy(verified bool, childURL, parentURL string) error {
	if !verified {
		return fmt.Errorf("%s is not a provider-native child of %s; sync it from the cycle that owns it", childURL, parentURL)
	}
	return nil
}
