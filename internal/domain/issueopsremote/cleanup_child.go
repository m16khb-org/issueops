package issueopsremote

import "strings"

// CleanupChildProvider picks the provider for child cleanup: the explicit value, else the one inferred from childURL, else from parentURL. It returns "" when none is known.
func CleanupChildProvider(explicit, childURL, parentURL string) string {
	for _, value := range []string{explicit, ProviderFromURL(childURL), ProviderFromURL(parentURL)} {
		if name := strings.TrimSpace(value); name != "" {
			return name
		}
	}
	return ""
}
