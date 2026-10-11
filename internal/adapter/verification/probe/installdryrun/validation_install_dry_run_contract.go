package installdryrun

import (
	"path/filepath"
	"slices"
)

var requiredInstallDryRunHosts = []string{"codex", "claude", "omo", "omp", "agy"}

func installDryRunValidationErrors(result installDryRunSmokeResult, tempHome, tempRoot string, pathExists func(string) bool) []string {
	errs := []string{}
	if !result.OK || !result.DryRun || !result.ProjectLocal {
		errs = append(errs, "install dry-run result flags mismatch")
	}
	if len(result.Hosts) != len(requiredInstallDryRunHosts) {
		errs = append(errs, "install dry-run host set mismatch")
	}
	errs = append(errs, installDryRunHostErrors(result.Hosts)...)
	if !slices.Contains(result.SkillNames, skillName) {
		errs = append(errs, "install dry-run did not discover smoke skill")
	}
	plannedErrs, plannedWrite, plannedLink := installDryRunPlanErrors(result)
	errs = append(errs, plannedErrs...)
	if !plannedWrite || !plannedLink {
		errs = append(errs, "install dry-run did not expose planned writes and links")
	}
	return append(errs, installDryRunUnexpectedPathErrors(tempHome, tempRoot, pathExists)...)
}

func installDryRunHostErrors(hosts []installDryRunSmokeHost) []string {
	var errs []string
	for index, host := range hosts {
		if !host.OK || !host.DryRun {
			errs = append(errs, "install dry-run host mismatch:"+host.Host)
		}
		if index >= len(requiredInstallDryRunHosts) || host.Host != requiredInstallDryRunHosts[index] {
			errs = append(errs, "install dry-run host order mismatch:"+host.Host)
		}
	}
	return errs
}

// installDryRunPlanErrors reports written/created entries and whether any write and link were planned.
func installDryRunPlanErrors(result installDryRunSmokeResult) (errs []string, plannedWrite, plannedLink bool) {
	for _, file := range result.Files {
		if file.Written {
			errs = append(errs, "install dry-run reported written file:"+file.Path)
		}
		if file.WouldWrite {
			plannedWrite = true
		}
	}
	for _, link := range result.Links {
		if link.Created {
			errs = append(errs, "install dry-run reported created link:"+link.Path)
		}
		if link.WouldCreate {
			plannedLink = true
		}
	}
	return errs, plannedWrite, plannedLink
}

func installDryRunUnexpectedPathErrors(tempHome, tempRoot string, pathExists func(string) bool) []string {
	var errs []string
	for _, path := range []string{
		filepath.Join(tempHome, ".codex"),
		filepath.Join(tempHome, ".claude"),
		filepath.Join(tempHome, ".omo"),
		filepath.Join(tempHome, ".omp"),
		filepath.Join(tempHome, ".gemini"),
		filepath.Join(tempRoot, "configs"),
		filepath.Join(tempRoot, ".mcp.json"),
		filepath.Join(tempRoot, ".claude"),
		filepath.Join(tempRoot, ".omo"),
		filepath.Join(tempRoot, ".omp"),
		filepath.Join(tempRoot, ".agents"),
	} {
		if pathExists(path) {
			errs = append(errs, "install dry-run wrote unexpected path:"+path)
		}
	}
	return errs
}
