package remote

import "strings"

func CleanupChildProvider(explicit, childURL, parentURL string) string {
	for _, value := range []string{explicit, ProviderFromURL(childURL), ProviderFromURL(parentURL)} {
		if name := strings.TrimSpace(value); name != "" {
			return name
		}
	}
	return ""
}
