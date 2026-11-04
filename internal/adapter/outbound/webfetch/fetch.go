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

// NewHTTPClient 함수는 client의 복제본을 감싸 webfetchport.HTTPClient로 돌려준다. 복제본은
// 리다이렉트를 따라가지 않고 마지막 응답을 그대로 돌려주므로, 리다이렉트 검증은 호출자가
// 한다. 응답 본문은 호출자가 닫는다.
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
	response, err := client.client.Do(httpRequest) //nolint:bodyclose // the body is returned to the caller, which owns closing it
	if err != nil {
		return webfetchport.HTTPResponse{}, err
	}
	return webfetchport.HTTPResponse{
		StatusCode: response.StatusCode,
		Header:     map[string][]string(response.Header),
		Body:       response.Body,
	}, nil
}

// ParseRetryAfter 함수는 Retry-After 헤더 값을 대기 시간으로 바꾼다. 초 단위 정수와 HTTP
// 날짜를 받아들이고, 비어 있거나 해석할 수 없으면 0을 돌려준다. 날짜가 과거이면 음수일 수
// 있다.
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
