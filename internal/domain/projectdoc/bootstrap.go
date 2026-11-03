package projectdoc

import "strings"

type BootstrapFileKind string

const (
	BootstrapRootFile     BootstrapFileKind = "root"
	BootstrapModuleFile   BootstrapFileKind = "module"
	BootstrapManifestFile BootstrapFileKind = "manifest"
)

type BootstrapFileInput struct {
	Kind   BootstrapFileKind
	Rel    string
	Action string
	Write  bool
	Sync   bool
}

type BootstrapFileDecision struct {
	Write           bool
	Preserved       bool
	Report          bool
	SyncWarning     bool
	FamilyPreserved bool
}

func DecideBootstrapFile(input BootstrapFileInput) BootstrapFileDecision {
	if input.Kind == BootstrapModuleFile || input.Kind == BootstrapManifestFile {
		return decideGeneratedBootstrapFile(input)
	}
	return decideRootBootstrapFile(input)
}

// decideGeneratedBootstrapFile covers module and manifest files, which are
// written only on create and never synced.
func decideGeneratedBootstrapFile(input BootstrapFileInput) BootstrapFileDecision {
	moduleUpdate := input.Kind == BootstrapModuleFile && input.Action == "update"
	return BootstrapFileDecision{
		Write:           input.Write && input.Action == "create",
		Preserved:       moduleUpdate,
		Report:          true,
		FamilyPreserved: moduleUpdate,
	}
}

func decideRootBootstrapFile(input BootstrapFileInput) BootstrapFileDecision {
	familyDoc := IsFamilyDocRel(input.Rel)
	shouldWrite := rootBootstrapWrite(input, familyDoc)
	return BootstrapFileDecision{
		Write:           shouldWrite,
		Preserved:       input.Action == "update" && (familyDoc || (input.Rel != "AGENTS.md" && !input.Sync)),
		Report:          true,
		SyncWarning:     !shouldWrite && input.Action == "update" && !input.Sync && !familyDoc,
		FamilyPreserved: familyDoc && input.Action == "update",
	}
}

// rootBootstrapWrite decides whether a root bootstrap file is written: family
// docs are written only on create, other files on create, sync, or AGENTS.md.
func rootBootstrapWrite(input BootstrapFileInput, familyDoc bool) bool {
	if familyDoc {
		return input.Write && input.Action == "create"
	}
	return input.Write && input.Action != "unchanged" &&
		(input.Sync || input.Action == "create" || input.Rel == "AGENTS.md")
}

func IsFamilyDocRel(rel string) bool {
	stripped := strings.TrimPrefix(rel, ProjectDocsDir+"/")
	if _, ok := FamilyByRoot(stripped); ok {
		return true
	}
	for _, family := range DocFamilies() {
		if strings.HasPrefix(stripped, family.ModuleDir+"/") {
			return true
		}
	}
	return false
}
