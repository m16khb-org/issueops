package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
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

var measurePipeCapacity = measureSystemPipeCapacity

var (
	probeMCPGateway    = probeMCPGatewayHTTP
	countMCPGatewayFDs = countMCPGatewayFDsViaLsof
)

func measureSystemPipeCapacity() (int, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return 0, err
	}
	defer r.Close()
	defer w.Close()

	progress := make(chan int, 64)
	done := make(chan error, 1)
	total := 0
	go func() {
		chunk := make([]byte, 512)
		for {
			n, err := w.Write(chunk)
			if n > 0 {
				progress <- n
			}
			if err != nil {
				done <- err
				return
			}
		}
	}()

	idle := time.NewTimer(100 * time.Millisecond)
	defer idle.Stop()
	for total < 1<<20 {
		select {
		case n := <-progress:
			total += n
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(100 * time.Millisecond)
		case err := <-done:
			if errors.Is(err, os.ErrClosed) || errors.Is(err, syscall.EPIPE) {
				return total, nil
			}
			return total, err
		case <-idle.C:
			_ = r.Close()
			_ = w.Close()
			select {
			case err := <-done:
				if err != nil && !errors.Is(err, os.ErrClosed) && !errors.Is(err, syscall.EPIPE) {
					return total, err
				}
			case <-time.After(time.Second):
			}
			return total, nil
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
	if os.IsNotExist(err) {
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

func probeMCPGatewayHTTP(target string) error {
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

func countMCPGatewayFDsViaLsof(port int) (int, error) {
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
