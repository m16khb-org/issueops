package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// MeasurePipeCapacity fills a fresh pipe with non-blocking writes until the
// kernel answers EAGAIN, so the buffered byte count is the pipe capacity. It
// returns in well under a millisecond instead of waiting on an idle timer.
func MeasurePipeCapacity() (int, error) {
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		return 0, err
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[1], true); err != nil {
		return 0, err
	}
	chunk := make([]byte, 512)
	total := 0
	for total < 1<<20 {
		n, err := syscall.Write(fds[1], chunk)
		if n > 0 {
			total += n
		}
		if errors.Is(err, syscall.EAGAIN) {
			return total, nil
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

type mcpGatewayEndpoint struct {
	Name string
	URL  *url.URL
}

func loopbackMCPEndpoints(configPath string) ([]mcpGatewayEndpoint, error) {
	raw, err := os.ReadFile(configPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var config struct {
		MCPServers map[string]struct {
			Type string `json:"type"`
			URL  string `json:"url"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	endpoints := []mcpGatewayEndpoint{}
	for name, server := range config.MCPServers {
		if server.Type != "http" && server.Type != "sse" {
			continue
		}
		parsed, err := url.Parse(server.URL)
		if err != nil {
			continue
		}
		host := parsed.Hostname()
		if host != "127.0.0.1" && host != "localhost" && host != "::1" {
			continue
		}
		endpoints = append(endpoints, mcpGatewayEndpoint{Name: name, URL: parsed})
	}
	sort.Slice(endpoints, func(i, j int) bool { return endpoints[i].Name < endpoints[j].Name })
	return endpoints, nil
}

func uniqueMCPGatewayPorts(endpoints []mcpGatewayEndpoint) []int {
	seen := map[int]bool{}
	ports := []int{}
	for _, ep := range endpoints {
		port := 80
		if ep.URL.Scheme == "https" {
			port = 443
		}
		if p := ep.URL.Port(); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil {
				port = parsed
			}
		}
		if !seen[port] {
			seen[port] = true
			ports = append(ports, port)
		}
	}
	sort.Ints(ports)
	return ports
}

func ProbeGatewayHTTP(target string) error {
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"issueops-doctor","version":"0"}}}`
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	// Any HTTP status proves the listener accepts and answers; a wedged
	// gateway fails at the transport layer (reset/refused/timeout) instead.
	return resp.Body.Close()
}

func CountGatewayFDsViaLsof(port int) (int, error) {
	pidOut, err := exec.Command("lsof", "-nP", fmt.Sprintf("-iTCP:%d", port), "-sTCP:LISTEN", "-t").Output()
	if err != nil {
		return 0, fmt.Errorf("listener pid lookup failed: %w", err)
	}
	pid := strings.TrimSpace(string(pidOut))
	if pid == "" {
		return 0, errors.New("no listener process found")
	}
	if i := strings.IndexByte(pid, '\n'); i >= 0 {
		pid = pid[:i]
	}
	fdOut, err := exec.Command("lsof", "-p", pid).Output()
	if err != nil {
		return 0, fmt.Errorf("fd listing failed for pid %s: %w", pid, err)
	}
	lines := strings.Count(string(fdOut), "\n")
	if lines > 0 {
		lines-- // drop the lsof header row
	}
	return lines, nil
}
