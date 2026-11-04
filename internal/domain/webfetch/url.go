package webfetch

import (
	"fmt"
	"net/url"
	"strings"
)

// SanitizeURL returns raw with any userinfo replaced by a placeholder so
// credentials never reach logs or output; an unparsable raw is returned
// trimmed.
func SanitizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	if parsed.User != nil {
		parsed.User = url.User("<redacted>")
	}
	return parsed.String()
}

// ResolveRedirect resolves the Location header value against the request URL
// base and returns the absolute target.
func ResolveRedirect(base, location string) (string, error) {
	parsedBase, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid redirect base: %w", err)
	}
	reference, err := url.Parse(location)
	if err != nil {
		return "", fmt.Errorf("invalid redirect location: %w", err)
	}
	return parsedBase.ResolveReference(reference).String(), nil
}

// IsRedirect reports whether status is an HTTP redirect code the fetcher
// follows (301, 302, 303, 307 or 308).
func IsRedirect(status int) bool {
	return status == 301 || status == 302 || status == 303 || status == 307 || status == 308
}
