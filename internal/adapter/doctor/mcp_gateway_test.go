package doctor

import (
	doctorcontract "issueops/internal/contract/doctor"
	doctordomain "issueops/internal/domain/doctor"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func writeClaudeMCPConfigForTest(t *testing.T, home, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

const loopbackGatewayConfigForTest = `{
	"mcpServers": {
		"glab-service-api": {"type": "http", "url": "http://127.0.0.1:7351/servers/service-api/mcp"},
		"glab-cloud-platform": {"type": "http", "url": "http://127.0.0.1:7351/servers/cloud-platform/mcp"},
		"remote-http": {"type": "http", "url": "https://example.com/mcp"},
		"local-stdio": {"command": "some-binary", "args": ["mcp"]}
	}
}`

func TestHarnessDoctorReportsMCPGatewayCheck(t *testing.T) {
	home := t.TempDir()
	writeClaudeMCPConfigForTest(t, home, loopbackGatewayConfigForTest)
	var mu sync.Mutex
	probed := []string{}
	oldProbe, oldCount := probeMCPGateway, countMCPGatewayFDs
	probeMCPGateway = func(target string) error {
		mu.Lock()
		probed = append(probed, target)
		mu.Unlock()
		return nil
	}
	countMCPGatewayFDs = fixedGatewayFDCountForTest(24)
	t.Cleanup(func() { probeMCPGateway, countMCPGatewayFDs = oldProbe, oldCount })

	result, err := HarnessDoctor(doctorcontract.HarnessDoctorRequest{RepoRoot: t.TempDir(), IssueOpsRoot: t.TempDir(), Home: home, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "mcp_gateway")
	if !ok || !check.Healthy {
		t.Fatalf("expected healthy mcp_gateway check, got check=%+v ok=%v issues=%+v", check, ok, result.Issues)
	}
	if len(probed) != 2 {
		t.Fatalf("expected 2 loopback endpoints probed (remote and stdio excluded), got %v", probed)
	}
	if !strings.Contains(check.Summary, "fd[:7351]=24") {
		t.Fatalf("expected fd count in summary, got %q", check.Summary)
	}
	if hasHarnessDoctorIssueForTest(result.Issues, "mcp_gateway_unreachable") || hasHarnessDoctorIssueForTest(result.Issues, "mcp_gateway_fd_pressure") {
		t.Fatalf("did not expect gateway issues: %+v", result.Issues)
	}
}

func TestHarnessDoctorWarnsOnUnreachableMCPGateway(t *testing.T) {
	home := t.TempDir()
	writeClaudeMCPConfigForTest(t, home, loopbackGatewayConfigForTest)
	oldProbe, oldCount := probeMCPGateway, countMCPGatewayFDs
	probeMCPGateway = func(target string) error { return os.ErrDeadlineExceeded }
	countMCPGatewayFDs = fixedGatewayFDCountForTest(24)
	t.Cleanup(func() { probeMCPGateway, countMCPGatewayFDs = oldProbe, oldCount })

	result, err := HarnessDoctor(doctorcontract.HarnessDoctorRequest{RepoRoot: t.TempDir(), IssueOpsRoot: t.TempDir(), Home: home, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "mcp_gateway")
	if !ok || check.Healthy {
		t.Fatalf("expected unhealthy mcp_gateway check, got check=%+v ok=%v", check, ok)
	}
	if !hasHarnessDoctorIssueForTest(result.Issues, "mcp_gateway_unreachable") {
		t.Fatalf("expected mcp_gateway_unreachable issue: %+v", result.Issues)
	}
}

func TestHarnessDoctorWarnsOnMCPGatewayFDPressure(t *testing.T) {
	home := t.TempDir()
	writeClaudeMCPConfigForTest(t, home, loopbackGatewayConfigForTest)
	oldProbe, oldCount := probeMCPGateway, countMCPGatewayFDs
	probeMCPGateway = func(target string) error { return nil }
	countMCPGatewayFDs = fixedGatewayFDCountForTest(doctordomain.MCPGatewayFDWarningThreshold)
	t.Cleanup(func() { probeMCPGateway, countMCPGatewayFDs = oldProbe, oldCount })

	result, err := HarnessDoctor(doctorcontract.HarnessDoctorRequest{RepoRoot: t.TempDir(), IssueOpsRoot: t.TempDir(), Home: home, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "mcp_gateway")
	if !ok || check.Healthy {
		t.Fatalf("expected unhealthy mcp_gateway check under fd pressure, got check=%+v ok=%v", check, ok)
	}
	if !hasHarnessDoctorIssueForTest(result.Issues, "mcp_gateway_fd_pressure") {
		t.Fatalf("expected mcp_gateway_fd_pressure issue: %+v", result.Issues)
	}
}

func TestHarnessDoctorSkipsMCPGatewayWithoutConfig(t *testing.T) {
	result, err := HarnessDoctor(doctorcontract.HarnessDoctorRequest{RepoRoot: t.TempDir(), IssueOpsRoot: t.TempDir(), Home: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	check, ok := harnessDoctorCheckForTest(result.Checks, "mcp_gateway")
	if !ok || !check.Healthy {
		t.Fatalf("expected healthy skip check without claude MCP config, got check=%+v ok=%v", check, ok)
	}
	if hasHarnessDoctorIssueForTest(result.Issues, "mcp_gateway_unreachable") {
		t.Fatalf("did not expect gateway issues without config: %+v", result.Issues)
	}
}

func fixedGatewayFDCountForTest(count int) func([]int) map[int]GatewayFDResult {
	return func(ports []int) map[int]GatewayFDResult {
		counted := make(map[int]GatewayFDResult, len(ports))
		for _, port := range ports {
			counted[port] = GatewayFDResult{Count: count}
		}
		return counted
	}
}
