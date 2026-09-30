package remote

import "fmt"

func ValidateChildLinkProvider(parentURL, childURL string) error {
	if parent := ProviderFromURL(parentURL); parent != "" && ProviderFromURL(childURL) != parent {
		return fmt.Errorf("child issue provider must match linked parent issue provider")
	}
	return nil
}
