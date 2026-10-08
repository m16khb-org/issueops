package issueopsapp

import (
	"os"
	"testing"
)

// TestMain wires the leaf CLI dependencies before running the package tests,
// mirroring what RunRootCommand does in production. Several tests call command
// runners (runInspect, runStatus, runDoctor, ...) directly without going through
// RunRootCommand and rely on the injected harness implementations.
func TestMain(m *testing.M) {
	// 개발자의 전역 agent model 설정을 읽지 않도록 빈 XDG 디렉터리를 쓴다.
	xdg, err := os.MkdirTemp("", "issueops-xdg-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_CONFIG_HOME", xdg)
	wireDependencies()
	os.Exit(m.Run())
}
