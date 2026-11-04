package mcpcli

import (
	"context"
	"io"
	"os"
	"os/signal"
	"syscall"
)

// RunMCPWithDependencies는 MCP 요청을 이 프로세스 안에서 처리한다. host는
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

func ServeMCPStreamWithDependencies(input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return ServeMCPStreamContextWithDependencies(context.Background(), input, output, diagnostics, deps)
}
