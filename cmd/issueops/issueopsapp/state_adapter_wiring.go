package issueopsapp

import (
	audit "issueops/internal/adapter/audit"
	channel "issueops/internal/adapter/channel"
	issueops "issueops/internal/adapter/issueops"
	statestore "issueops/internal/adapter/outbound/state"
	trace "issueops/internal/adapter/trace"
)

// configureAdapterStateAccess는 outbound state를 쓰는 adapter들에 접근자를 설치한다.
//
// adapter가 다른 adapter의 package 함수를 직접 부르면 capability 경계를 넘는 결합이
// 된다. state store를 아는 곳은 composition root 하나여야 한다.
func configureAdapterStateAccess() {
	audit.StateDir = statestore.StateDir
	audit.WithKeyLock = statestore.WithKeyLock
	issueops.StateDir = statestore.StateDir
	channel.StateDir = statestore.StateDir
	trace.StateRead = statestore.StateRead
}
