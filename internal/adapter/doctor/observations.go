package doctor

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"issueops/internal/adapter/outbound/osfile"
	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/projectdoc"
)

func ObserveProjectDocs(root string) doctordomain.ProjectDocsObservation {
	observation := doctordomain.ProjectDocsObservation{Directory: filepath.Join(root, projectdoc.ProjectDocsDir)}
	for _, name := range projectdoc.ProjectDocNames() {
		if _, err := os.Stat(filepath.Join(observation.Directory, name)); errors.Is(err, fs.ErrNotExist) {
			observation.Missing = append(observation.Missing, name)
		}
	}
	return observation
}

func ObserveRuntimeState(root string) doctordomain.RuntimeStateObservation {
	observation := doctordomain.RuntimeStateObservation{DocumentPath: filepath.Join(root, projectdoc.ProjectDocsDir, "STATE.md")}
	for _, path := range []string{filepath.Join(root, projectdoc.ProjectDocsDir, "state"), filepath.Join(root, projectdoc.ProjectDocsDir, "state.schema.json")} {
		if _, err := os.Stat(path); err == nil {
			observation.Paths = append(observation.Paths, path)
		}
	}
	if content, err := os.ReadFile(observation.DocumentPath); err == nil {
		observation.Document = string(content)
	}
	return observation
}

// maxConcurrentGatewayProbes bounds the loopback HTTP probes in flight; a doctor
// run sees a handful of gateways, so this only guards a pathological config.
const maxConcurrentGatewayProbes = 8

// Gateways observes the loopback MCP gateways. Probe and the FD counter are
// injected so tests stay hermetic. CountFDs counts every port's listener FDs in
// one call.
type Gateways struct {
	Probe    func(string) error
	CountFDs func([]int) map[int]GatewayFDResult
}

func (gateways Gateways) Observe(home string) doctordomain.GatewayObservation {
	observation := doctordomain.GatewayObservation{Home: home}
	if home == "" {
		return observation
	}
	endpoints, err := loopbackMCPEndpoints(filepath.Join(home, ".claude.json"))
	if err != nil {
		observation.ConfigError = err
		return observation
	}
	ports := uniqueMCPGatewayPorts(endpoints)
	var fdResults []GatewayFDResult
	var fds sync.WaitGroup
	fds.Go(func() { fdResults = gateways.countFDs(ports) })
	observation.Endpoints = gateways.probeEndpoints(endpoints)
	fds.Wait()
	for i, port := range ports {
		observation.FDs = append(observation.FDs, doctordomain.GatewayFD{Port: port, Count: fdResults[i].Count, Available: fdResults[i].Err == nil})
	}
	return observation
}

// probeEndpoints probes the endpoints concurrently, at most
// maxConcurrentGatewayProbes at a time, and returns them in endpoint order.
func (gateways Gateways) probeEndpoints(endpoints []mcpGatewayEndpoint) []doctordomain.GatewayEndpoint {
	var observed []doctordomain.GatewayEndpoint
	if len(endpoints) == 0 {
		return observed
	}
	observed = make([]doctordomain.GatewayEndpoint, len(endpoints))
	slots := make(chan struct{}, maxConcurrentGatewayProbes)
	var probes sync.WaitGroup
	for i, endpoint := range endpoints {
		slots <- struct{}{}
		probes.Go(func() {
			defer func() { <-slots }()
			target := endpoint.URL.String()
			observed[i] = doctordomain.GatewayEndpoint{Name: endpoint.Name, URL: target, Error: gateways.Probe(target)}
		})
	}
	probes.Wait()
	return observed
}

// countFDs returns one result per port, in port order.
func (gateways Gateways) countFDs(ports []int) []GatewayFDResult {
	results := make([]GatewayFDResult, len(ports))
	if len(ports) == 0 {
		return results
	}
	counted := gateways.CountFDs(ports)
	for i, port := range ports {
		result, ok := counted[port]
		if !ok {
			result = GatewayFDResult{Err: errors.New("no fd count for port")}
		}
		results[i] = result
	}
	return results
}

func ObserveNativeIntegrations(home string) doctordomain.NativeObservation {
	observation := doctordomain.NativeObservation{Home: home}
	if home == "" {
		return observation
	}
	observation.HooksPath = filepath.Join(home, ".codex", "hooks.json")
	_, err := os.Stat(observation.HooksPath)
	observation.HooksMissing = errors.Is(err, fs.ErrNotExist)
	return observation
}

// ObserveBinaryDrift compares bin/issueops with the newest production Go
// source under cmd/ and internal/. Test files and testdata are not compiled
// into the binary, so they neither make it stale nor cost an lstat each; the
// walk uses directory entries and stats only the production .go files.
func ObserveBinaryDrift(root string) doctordomain.BinaryObservation {
	observation := doctordomain.BinaryObservation{Root: root}
	if root == "" {
		return observation
	}
	observation.Path = filepath.Join(root, "bin", osfile.ExecutableName("issueops"))
	info, err := os.Stat(observation.Path)
	if err != nil {
		return observation
	}
	observation.Found = true
	observation.BuiltAt = info.ModTime()
	observation.LatestSource = observation.BuiltAt
	for _, directory := range []string{filepath.Join(root, "cmd"), filepath.Join(root, "internal")} {
		_ = filepath.WalkDir(directory, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			name := entry.Name()
			if entry.IsDir() {
				if name == "testdata" {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				return nil
			}
			info, err := entry.Info()
			if err == nil && info.ModTime().After(observation.LatestSource) {
				observation.LatestSource = info.ModTime()
			}
			return nil
		})
	}
	return observation
}
