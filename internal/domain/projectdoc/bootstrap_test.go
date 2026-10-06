package projectdoc

import "testing"

func TestBootstrapFileDecisionPreservesCuratedDocs(t *testing.T) {
	for _, tc := range []struct {
		name                                                   string
		input                                                  BootstrapFileInput
		write, preserved, report, syncWarning, familyPreserved bool
	}{
		{name: "curated standard", input: BootstrapFileInput{Kind: BootstrapRootFile, Rel: ".issueops/TECH_STACK.md", Action: "update", Write: true}, preserved: true, report: true, syncWarning: true},
		{name: "synced standard", input: BootstrapFileInput{Kind: BootstrapRootFile, Rel: ".issueops/TECH_STACK.md", Action: "update", Write: true, Sync: true}, write: true, report: true},
		{name: "agents routing", input: BootstrapFileInput{Kind: BootstrapRootFile, Rel: "AGENTS.md", Action: "update", Write: true}, write: true, report: true},
		{name: "family root", input: BootstrapFileInput{Kind: BootstrapRootFile, Rel: ".issueops/ADR.md", Action: "update", Write: true, Sync: true}, preserved: true, report: true, familyPreserved: true},
		{name: "family module create", input: BootstrapFileInput{Kind: BootstrapModuleFile, Rel: ".issueops/adr/overview.md", Action: "create", Write: true}, write: true, report: true},
		{name: "family module update", input: BootstrapFileInput{Kind: BootstrapModuleFile, Rel: ".issueops/adr/overview.md", Action: "update", Write: true, Sync: true}, preserved: true, report: true, familyPreserved: true},
		{name: "family root create", input: BootstrapFileInput{Kind: BootstrapRootFile, Rel: ".issueops/ADR.md", Action: "create", Write: true}, write: true, report: true},
		{name: "manifest update", input: BootstrapFileInput{Kind: BootstrapManifestFile, Rel: ".issueops/documentation/manifest.json", Action: "update", Write: true, Sync: true}, report: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := DecideBootstrapFile(tc.input)
			if got.Write != tc.write || got.Preserved != tc.preserved || got.Report != tc.report || got.SyncWarning != tc.syncWarning || got.FamilyPreserved != tc.familyPreserved {
				t.Fatalf("decision=%+v", got)
			}
		})
	}
}
