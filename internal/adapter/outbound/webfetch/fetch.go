package webfetch

import (
	"context"
	webfetchport "issueops/internal/port/webfetch"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func cloneNoRedirectClient(client *http.Client) *http.Client {
	clone := *client
	clone.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clone
}

type netHTTPClient struct {
	client *http.Client
}

func NewHTTPClient(client *http.Client) webfetchport.HTTPClient {
	return netHTTPClient{client: cloneNoRedirectClient(client)}
}

func (client netHTTPClient) Do(ctx context.Context, request webfetchport.HTTPRequest) (webfetchport.HTTPResponse, error) {
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, request.URL, nil)
	if err != nil {
		return webfetchport.HTTPResponse{}, err
	}
	for key, value := range request.Header {
		httpRequest.Header.Set(key, value)
	}
	response, err := client.client.Do(httpRequest)
	if err != nil {
		return webfetchport.HTTPResponse{}, err
	}
	return webfetchport.HTTPResponse{
		StatusCode: response.StatusCode,
		Header:     map[string][]string(response.Header),
		Body:       response.Body,
	}, nil
}

func ParseRetryAfter(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		return time.Duration(seconds) * time.Second
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0
	}
	return time.Until(when)
}
