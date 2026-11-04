// Package mcpcli 패키지는 issueops MCP 서버의 CLI 계층이다. stdio와 loopback HTTP 전송, 도구 카탈로그 디스패치,
// 요청별 workspace 권한 검증을 맡고, 실제 처리는 주입받은 application 서비스에 위임한다.
package mcpcli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// RunMCPWithDependencies 함수는 MCP 요청을 이 프로세스 안에서 처리한다. host는
// `issueops mcp`를 세션마다 자식 프로세스로 띄우므로, issueops_execution이
// 관측하는 프로세스 계보에 호출한 세션이 들어가 native actor 증명이 성립한다.
// 상태 권위는 SQLite라 요청을 한 프로세스로 모을 필요도 없다.
//
// SIGINT와 SIGTERM은 HTTP 서비스처럼 처리 중인 요청을 기다린 뒤(최대
// httpShutdownDrain) 취소하고 종료 코드 0으로 끝낸다. 첫 신호 뒤에는 기본 처리를
// 되돌려, 두 번째 신호는 바로 프로세스를 끝낸다.
func RunMCPWithDependencies(deps MCPDependencies) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer context.AfterFunc(ctx, stop)()
	return ServeMCPStreamContextWithDependencies(ctx, os.Stdin, os.Stdout, os.Stderr, deps)
}
