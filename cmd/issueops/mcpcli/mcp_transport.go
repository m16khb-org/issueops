package mcpcli

import (
	"context"
	"io"
	"os"
)

// RunMCPWithDependencies는 MCP 요청을 이 프로세스 안에서 처리한다. host는
// `issueops mcp`를 세션마다 자식 프로세스로 띄우므로, issueops_execution이
// 관측하는 프로세스 계보에 호출한 세션이 들어간다. 공유 daemon으로 proxy하면
// 계보가 daemon의 것이 되어 native actor 증명이 성립하지 않는다. 상태 권위는
// SQLite라 요청을 한 프로세스로 모을 필요도 없다.
func RunMCPWithDependencies(deps MCPDependencies) error {
	return ServeMCPStreamWithDependencies(os.Stdin, os.Stdout, os.Stderr, deps)
}

func ServeMCPStreamWithDependencies(input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return ServeMCPStreamContextWithDependencies(context.Background(), input, output, diagnostics, deps)
}

func ServeMCPStreamContext(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer) error {
	return ServeMCPStreamContextWithDependencies(ctx, input, output, diagnostics, MCPDependencies{})
}

func ServeMCPStreamContextWithDependencies(ctx context.Context, input io.Reader, output io.Writer, diagnostics io.Writer, deps MCPDependencies) error {
	return serveMCPStreamSDK(ctx, input, output, diagnostics, deps)
}
