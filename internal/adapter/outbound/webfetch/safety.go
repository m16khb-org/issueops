package webfetch

import (
	"context"
	"net"
)

type NetResolver struct{}

func (NetResolver) LookupIPAddr(ctx context.Context, host string) ([]string, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(addresses))
	for _, address := range addresses {
		result = append(result, address.IP.String())
	}
	return result, nil
}
