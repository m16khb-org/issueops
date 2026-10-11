package webfetch

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	webfetchcontract "issueops/internal/contract/webfetch"
	webfetchdomain "issueops/internal/domain/webfetch"
	webfetchport "issueops/internal/port/webfetch"
)

const maxRedirects = 8

type Dependencies struct {
	HTTPClient webfetchport.HTTPClient
	URLPolicy  webfetchport.URLPolicy
	Now        func() time.Time
	RetryAfter func(value string) time.Duration
	Sleep      func(context.Context, time.Duration)
}

func Fetch(ctx context.Context, request webfetchcontract.Request, dependencies Dependencies) (webfetchcontract.Result, error) {
	started := time.Now()
	now := started
	if dependencies.Now != nil {
		now = dependencies.Now()
	}
	result := newFetchResult(request, now)
	timeout := requestTimeout(request)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if dependencies.URLPolicy == nil || dependencies.HTTPClient == nil {
		return result, fmt.Errorf("web fetch dependencies are required")
	}
	current, err := dependencies.URLPolicy.Validate(ctx, request.URL, request.AllowPrivateNetwork)
	if err != nil {
		result.Category = webfetchcontract.CategoryBlocked
		result.StopReason = webfetchcontract.StopReasonSafetyRejected
		result.GridExhausted = true
		result.Warnings = append(result.Warnings, err.Error())
		result.UntriedRoutes = SkippedRoutes("safety_rejected")
		return FinalizeResult(result, request.MaxChars, started), nil
	}
	result.FinalURL = webfetchdomain.SanitizeURL(current)
	result.AttemptedRoutes = append(result.AttemptedRoutes, webfetchcontract.RouteRecord{ID: "direct_http", Status: "attempted"})

	visited := map[string]bool{}
	retriedAfter := false
	for redirects := 0; redirects <= maxRedirects; redirects++ {
		if visited[current] {
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonRedirectLoop, "redirect_loop"), nil
		}
		visited[current] = true
		response, requestErr := dependencies.HTTPClient.Do(ctx, webfetchport.HTTPRequest{
			URL:    current,
			Header: map[string]string{"User-Agent": "issueops-web-fetch/1.0"},
		})
		if requestErr != nil {
			result.Warnings = append(result.Warnings, requestErr.Error())
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonError, "request_error"), nil
		}

		if webfetchdomain.IsRedirect(response.StatusCode) {
			location := webfetchdomain.HeaderValue(response.Header, "Location")
			_ = response.Body.Close()
			next, stop := followRedirect(ctx, dependencies, request, current, location)
			if stop != nil {
				if stop.err != nil {
					result.Warnings = append(result.Warnings, stop.err.Error())
				}
				return stopped(result, request.MaxChars, started, stop.category, stop.reason, stop.skipped), nil
			}
			current = next
			result.FinalURL = webfetchdomain.SanitizeURL(current)
			continue
		}

		body, readErr := io.ReadAll(io.LimitReader(response.Body, 2*1024*1024))
		_ = response.Body.Close()
		if readErr != nil {
			result.Warnings = append(result.Warnings, readErr.Error())
			return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonError, "read_error"), nil
		}
		validation := webfetchdomain.ValidateResponse(webfetchcontract.ResponseValidationInput{
			StatusCode: response.StatusCode,
			Header:     response.Header,
			Body:       body,
			URL:        current,
		})
		if retryAfter := webfetchdomain.HeaderValue(response.Header, "Retry-After"); shouldRetryAfter(validation, retryAfter, retriedAfter) {
			retriedAfter = true
			serviceRetryDelay(ctx, dependencies, retryAfter)
			delete(visited, current)
			continue
		}
		result = applyValidation(result, validation, retriedAfter)
		return FinalizeResult(result, request.MaxChars, started), nil
	}
	return stopped(result, request.MaxChars, started, webfetchcontract.CategoryUnknown, webfetchcontract.StopReasonRedirectLoop, "redirect_loop"), nil
}

func requestTimeout(request webfetchcontract.Request) time.Duration {
	if request.Timeout <= 0 {
		return 30 * time.Second
	}
	return request.Timeout
}

// shouldRetryAfter allows exactly one retry for a rate-limited response that carries a Retry-After header.
func shouldRetryAfter(validation webfetchcontract.ResponseValidation, retryAfter string, alreadyRetried bool) bool {
	return validation.Category == webfetchcontract.CategoryRateLimited && retryAfter != "" && !alreadyRetried
}

func newFetchResult(request webfetchcontract.Request, now time.Time) webfetchcontract.Result {
	return webfetchcontract.Result{
		URL:             webfetchdomain.SanitizeURL(request.URL),
		Category:        webfetchcontract.CategoryUnknown,
		StopReason:      webfetchcontract.StopReasonError,
		AttemptedRoutes: []webfetchcontract.RouteRecord{},
		UntriedRoutes:   []webfetchcontract.RouteRecord{},
		Metadata:        map[string]any{},
		Warnings:        []string{},
		RetrievedAt:     now.UTC().Format(time.RFC3339Nano),
	}
}

// redirectStop describes why a redirect hop ended the fetch; err's message is appended to the result warnings when err is set.
type redirectStop struct {
	category, reason, skipped string
	err                       error
}

// followRedirect resolves and validates a redirect Location. A non-nil stop means the fetch must end.
func followRedirect(ctx context.Context, dependencies Dependencies, request webfetchcontract.Request, current, location string) (string, *redirectStop) {
	if strings.TrimSpace(location) == "" {
		return "", &redirectStop{category: webfetchcontract.CategoryUnknown, reason: webfetchcontract.StopReasonGridExhausted, skipped: "empty_redirect_location"}
	}
	next, resolveErr := webfetchdomain.ResolveRedirect(current, location)
	if resolveErr != nil {
		return "", &redirectStop{category: webfetchcontract.CategoryUnknown, reason: webfetchcontract.StopReasonUnsafeRedirect, skipped: "unsafe_redirect", err: resolveErr}
	}
	validated, validateErr := dependencies.URLPolicy.Validate(ctx, next, request.AllowPrivateNetwork)
	if validateErr != nil {
		return "", &redirectStop{category: webfetchcontract.CategoryBlocked, reason: webfetchcontract.StopReasonUnsafeRedirect, skipped: "unsafe_redirect", err: validateErr}
	}
	return validated, nil
}

// applyValidation records a final (non-redirect) response validation on the result.
func applyValidation(result webfetchcontract.Result, validation webfetchcontract.ResponseValidation, retriedAfter bool) webfetchcontract.Result {
	result.Category = validation.Category
	result.StopReason = validation.StopReason
	result.Content = validation.Content
	result.Metadata = validation.Metadata
	if retriedAfter {
		result.Metadata["retry_after_retried"] = true
	}
	result.Warnings = append(result.Warnings, validation.Warnings...)
	result.OK = validation.Category == webfetchcontract.CategoryStrongOK || validation.Category == webfetchcontract.CategoryWeakOK || validation.Category == webfetchcontract.CategoryPaywalled
	if validation.StopReason == webfetchcontract.StopReasonAccepted {
		result.GridExhausted = false
		result.UntriedRoutes = SkippedRoutes("not_needed_after_success")
	} else {
		result.GridExhausted = true
		result.UntriedRoutes = SkippedRoutes(validation.Category)
	}
	return result
}

func stopped(result webfetchcontract.Result, maxChars int, started time.Time, category, reason, skippedReason string) webfetchcontract.Result {
	result.Category = category
	result.StopReason = reason
	result.GridExhausted = true
	result.UntriedRoutes = SkippedRoutes(skippedReason)
	return FinalizeResult(result, maxChars, started)
}

func serviceRetryDelay(ctx context.Context, dependencies Dependencies, value string) {
	if dependencies.RetryAfter == nil {
		return
	}
	delay := dependencies.RetryAfter(value)
	if delay <= 0 {
		return
	}
	if delay > 100*time.Millisecond {
		delay = 100 * time.Millisecond
	}
	if dependencies.Sleep != nil {
		dependencies.Sleep(ctx, delay)
		return
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}
