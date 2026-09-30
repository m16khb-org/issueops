package webfetch

import (
	"context"
	"fmt"
	domain "issueops/internal/domain/webfetch"
	port "issueops/internal/port/webfetch"
	"strings"
)

type URLValidator struct{ Resolver port.Resolver }

func (v URLValidator) Validate(ctx context.Context, raw string, allowPrivate bool) (string, error) {
	parsed, err := domain.ParseFetchURL(raw, allowPrivate)
	if err != nil {
		return "", err
	}
	host := strings.TrimSpace(parsed.Hostname())
	if !domain.IsIPAddress(strings.Trim(host, "[]")) {
		addresses, err := v.Resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return "", fmt.Errorf("host resolution failed for %q: %w", host, err)
		}
		for _, address := range addresses {
			if domain.IsUnsafeHost(address, allowPrivate) {
				return "", fmt.Errorf("unsafe resolved host %q -> %s", host, address)
			}
		}
	}
	return parsed.String(), nil
}
