package doctor

import (
	"os"
	"path/filepath"
	"strings"

	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/projectdoc"
)

func ObserveProjectDocs(root string) doctordomain.ProjectDocsObservation {
	observation := doctordomain.ProjectDocsObservation{Directory: filepath.Join(root, projectdoc.ProjectDocsDir)}
	for _, name := range projectdoc.ProjectDocNames() {
		if _, err := os.Stat(filepath.Join(observation.Directory, name)); os.IsNotExist(err) {
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

type Gateways struct {
	Probe    func(string) error
	CountFDs func(int) (int, error)
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
	for _, endpoint := range endpoints {
		target := endpoint.URL.String()
		observation.Endpoints = append(observation.Endpoints, doctordomain.GatewayEndpoint{Name: endpoint.Name, URL: target, Error: gateways.Probe(target)})
	}
	for _, port := range uniqueMCPGatewayPorts(endpoints) {
		count, err := gateways.CountFDs(port)
		observation.FDs = append(observation.FDs, doctordomain.GatewayFD{Port: port, Count: count, Available: err == nil})
	}
	return observation
}

func ObserveNativeIntegrations(home string) doctordomain.NativeObservation {
	observation := doctordomain.NativeObservation{Home: home}
	if home == "" {
		return observation
	}
	observation.HooksPath = filepath.Join(home, ".codex", "hooks.json")
	_, err := os.Stat(observation.HooksPath)
	observation.HooksMissing = os.IsNotExist(err)
	return observation
}

func ObserveBinaryDrift(root string) doctordomain.BinaryObservation {
	observation := doctordomain.BinaryObservation{Root: root}
	if root == "" {
		return observation
	}
	observation.Path = filepath.Join(root, "bin", "issueops")
	info, err := os.Stat(observation.Path)
	if err != nil {
		return observation
	}
	observation.Found = true
	observation.BuiltAt = info.ModTime()
	observation.LatestSource = observation.BuiltAt
	for _, directory := range []string{filepath.Join(root, "cmd"), filepath.Join(root, "internal")} {
		_ = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(info.Name(), ".go") {
				return nil
			}
			if info.ModTime().After(observation.LatestSource) {
				observation.LatestSource = info.ModTime()
			}
			return nil
		})
	}
	return observation
}
