package webfetch

import (
	"context"
	"net"
)

// NetResolver 타입은 기본 resolver로 호스트 이름을 IP 주소로 해석한다.
type NetResolver struct{}

// LookupIPAddr 메서드는 host를 해석해 IP 주소 문자열 목록으로 돌려준다.
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
