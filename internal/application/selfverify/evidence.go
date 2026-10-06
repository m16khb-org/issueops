package selfverify

import (
	"encoding/json"
	"fmt"
	"io"
	contract "issueops/internal/contract/selfverify"
	"strings"
)

func requireGoldenEvidence(step contract.StepResult) error {
	if step.StdoutTruncated {
		return fmt.Errorf("truncated go test JSON")
	}
	required := map[string]bool{
		"issueops/cmd/issueops/contractgolden/TestCLIUsageGolden":       false,
		"issueops/cmd/issueops/contractgolden/TestMCPToolsGolden":       false,
		"issueops/cmd/issueops/contractgolden/TestMCPResourcesGolden":   false,
		"issueops/cmd/issueops/issueopsapp/TestResponseContractsGolden": false,
	}
	ran := make(map[string]bool)
	passedPackages := make(map[string]bool)
	decoder := json.NewDecoder(strings.NewReader(step.Stdout))
	for {
		var event struct{ Action, Package, Test string }
		err := decoder.Decode(&event)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid go test JSON: %w", err)
		}
		if event.Action == "fail" || event.Action == "skip" {
			return fmt.Errorf("%s: %s %s", event.Action, event.Package, event.Test)
		}
		if event.Test == "" {
			passedPackages[event.Package] = event.Action == "pass"
			continue
		}
		key := event.Package + "/" + event.Test
		if _, expected := required[key]; !expected {
			continue
		}
		if event.Action == "run" {
			ran[key] = true
		}
		if event.Action == "pass" && ran[key] {
			required[key] = true
		}
	}
	for test, passed := range required {
		if !passed {
			return fmt.Errorf("required test did not run and pass: %s", test)
		}
	}
	for _, pkg := range []string{"issueops/cmd/issueops/contractgolden", "issueops/cmd/issueops/issueopsapp"} {
		if !passedPackages[pkg] {
			return fmt.Errorf("package did not pass: %s", pkg)
		}
	}

	return nil
}

func binaryDriftEvidence(step contract.StepResult) contract.StepResult {
	if !step.OK {
		return step
	}
	var result struct {
		Checks []struct {
			Name    string `json:"name"`
			Healthy *bool  `json:"healthy"`
			Summary string `json:"summary"`
		} `json:"checks"`
	}
	if step.StdoutTruncated {
		step.Error = "binary drift evidence: truncated doctor JSON"
	} else if err := json.Unmarshal([]byte(step.Stdout), &result); err != nil {
		step.Error = "binary drift evidence: invalid doctor JSON: " + err.Error()
	} else {
		count := 0
		for _, check := range result.Checks {
			if check.Name != "binary_drift" {
				continue
			}
			count++
			if check.Healthy == nil {
				step.Error = "binary drift evidence: missing healthy boolean"
			} else if !*check.Healthy {
				step.Error = "binary drift: " + check.Summary
				if check.Summary == "" {
					step.Error = "binary drift: unhealthy"
				}
			}
		}
		if count != 1 {
			step.Error = fmt.Sprintf("binary drift evidence: expected one binary_drift check, got %d", count)
		}
	}
	step.OK = step.Error == ""
	return step
}
