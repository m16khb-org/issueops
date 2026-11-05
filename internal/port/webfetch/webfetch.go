// Package webfetch declares the ports the web fetch application needs: an HTTP
// client, a DNS resolver, and a URL policy. Adapters supply the network effects so
// the fetch rules stay testable.
package webfetch

import (
	"context"
	"io"
)

// HTTPClient performs one HTTP request without following redirects on its own;
// the caller validates and follows each hop.
type HTTPClient interface {
	Do(context.Context, HTTPRequest) (HTTPResponse, error)
}

// Resolver resolves a host name to its IP addresses as strings.
type Resolver interface {
	LookupIPAddr(context.Context, string) ([]string, error)
}

// URLPolicy validates a URL before it is fetched. The boolean allows private
// network targets, and the returned string is the accepted URL to request.
type URLPolicy interface {
	Validate(context.Context, string, bool) (string, error)
}

// HTTPRequest is one outbound GET: the target URL and the request headers.
type HTTPRequest struct {
	URL    string
	Header map[string]string
}

// HTTPResponse is the status, headers, and streaming body of a fetched response.
// The caller closes Body.
type HTTPResponse struct {
	StatusCode int
	Header     map[string][]string
	Body       io.ReadCloser
}
