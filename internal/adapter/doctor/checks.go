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
	"time"
)

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

// GatewayFDResult is one port's lsof answer: the listener's open-FD count, or
// the reason the count is unavailable.
type GatewayFDResult struct {
	Count int
	Err   error
}

// CountGatewayFDsViaLsof counts the FDs of each port's listener with two lsof
// calls in total, one listener scan and one FD listing over every listener pid,
// instead of two calls per port. The first listener pid wins for a port with
// several listeners.
func CountGatewayFDsViaLsof(ports []int) map[int]GatewayFDResult {
	return countGatewayFDsBatch(ports, runLsof)
}

func runLsof(args ...string) ([]byte, error) {
	return exec.Command("lsof", args...).Output()
}

func countGatewayFDsBatch(ports []int, run func(args ...string) ([]byte, error)) map[int]GatewayFDResult {
	results := make(map[int]GatewayFDResult, len(ports))
	if len(ports) == 0 {
		return results
	}
	listenOut, err := run("-nP", "-iTCP", "-sTCP:LISTEN", "-Fpn")
	if err != nil && len(listenOut) == 0 {
		for _, port := range ports {
			results[port] = GatewayFDResult{Err: fmt.Errorf("listener pid lookup failed: %w", err)}
		}
		return results
	}
	listeners := parseLsofListeners(listenOut)
	pidByPort, pids := resolveGatewayListenerPIDs(ports, listeners, results)
	if len(pids) == 0 {
		return results
	}
	fdOut, err := run("-p", strings.Join(pids, ","), "-Fpf")
	if err != nil && len(fdOut) == 0 {
		for port, pid := range pidByPort {
			results[port] = GatewayFDResult{Err: fmt.Errorf("fd listing failed for pid %d: %w", pid, err)}
		}
		return results
	}
	counts := parseLsofFDCounts(fdOut)
	recordGatewayFDCounts(pidByPort, counts, results)
	return results
}

// resolveGatewayListenerPIDs records a "no listener" result for each port
// without a listener and returns the port-to-pid map plus the de-duplicated
// pid list in first-seen port order.
func resolveGatewayListenerPIDs(ports []int, listeners map[int]int, results map[int]GatewayFDResult) (map[int]int, []string) {
	pidByPort := make(map[int]int, len(ports))
	pidSeen := map[int]bool{}
	pids := []string{}
	for _, port := range ports {
		pid, ok := listeners[port]
		if !ok {
			results[port] = GatewayFDResult{Err: errors.New("no listener process found")}
			continue
		}
		pidByPort[port] = pid
		if !pidSeen[pid] {
			pidSeen[pid] = true
			pids = append(pids, strconv.Itoa(pid))
		}
	}
	return pidByPort, pids
}

// recordGatewayFDCounts stores each port's FD count, or an error result when
// lsof reported no records for its listener pid.
func recordGatewayFDCounts(pidByPort, counts map[int]int, results map[int]GatewayFDResult) {
	for port, pid := range pidByPort {
		count, ok := counts[pid]
		if !ok {
			results[port] = GatewayFDResult{Err: fmt.Errorf("fd listing failed for pid %d: no lsof records", pid)}
			continue
		}
		results[port] = GatewayFDResult{Count: count}
	}
}

// parseLsofListeners maps each listening port to the first pid lsof reported
// for it, from `lsof -Fpn` records (`p<pid>` then `n<addr>:<port>`).
func parseLsofListeners(out []byte) map[int]int {
	listeners := map[int]int{}
	pid := 0
	for line := range strings.SplitSeq(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			parsed, err := strconv.Atoi(line[1:])
			if err != nil {
				pid = 0
				continue
			}
			pid = parsed
		case strings.HasPrefix(line, "n") && pid != 0:
			colon := strings.LastIndexByte(line, ':')
			if colon < 0 {
				continue
			}
			port, err := strconv.Atoi(line[colon+1:])
			if err != nil {
				continue
			}
			if _, ok := listeners[port]; !ok {
				listeners[port] = pid
			}
		}
	}
	return listeners
}

// parseLsofFDCounts counts open files per pid from `lsof -Fpf` records. A pid
// lsof listed without files maps to zero; a pid it did not list is absent.
func parseLsofFDCounts(out []byte) map[int]int {
	counts := map[int]int{}
	pid := 0
	for line := range strings.SplitSeq(string(out), "\n") {
		switch {
		case strings.HasPrefix(line, "p"):
			parsed, err := strconv.Atoi(line[1:])
			if err != nil {
				pid = 0
				continue
			}
			pid = parsed
			counts[pid] += 0
		case strings.HasPrefix(line, "f") && pid != 0:
			counts[pid]++
		}
	}
	return counts
}
