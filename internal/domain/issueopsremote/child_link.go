package issueopsremote

import "fmt"

// ValidateChildLinkProvider requires childURL to resolve to the same provider as parentURL. A parent whose provider is unrecognised imposes no constraint.
func ValidateChildLinkProvider(parentURL, childURL string) error {
	if parent := ProviderFromURL(parentURL); parent != "" && ProviderFromURL(childURL) != parent {
		return fmt.Errorf("child issue provider must match linked parent issue provider")
	}
	return nil
}
