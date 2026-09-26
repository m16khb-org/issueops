package projectdoc

import "strings"

type BootstrapFileKind string

const (
	BootstrapRootFile     BootstrapFileKind = "root"
	BootstrapModuleFile   BootstrapFileKind = "module"
	BootstrapManifestFile BootstrapFileKind = "manifest"
)

type BootstrapFileInput struct {
	Kind       BootstrapFileKind
	Rel        string
	Action     string
	Write      bool
	Sync       bool
	LegacyFlat bool
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
		if input.LegacyFlat {
			return BootstrapFileDecision{FamilyPreserved: input.Kind == BootstrapModuleFile && input.Action == "update"}
		}
		return BootstrapFileDecision{
			Write:           input.Write && input.Action == "create",
			Preserved:       input.Kind == BootstrapModuleFile && input.Action == "update",
			Report:          true,
			FamilyPreserved: input.Kind == BootstrapModuleFile && input.Action == "update",
		}
	}
	familyDoc := IsFamilyDocRel(input.Rel)
	shouldWrite := input.Write && input.Action != "unchanged" && !familyDoc &&
		(input.Sync || input.Action == "create" || input.Rel == "AGENTS.md")
	if familyDoc && input.Action == "create" && !input.LegacyFlat {
		shouldWrite = input.Write
	}
	return BootstrapFileDecision{
		Write:           shouldWrite,
		Preserved:       input.Action == "update" && (familyDoc || (input.Rel != "AGENTS.md" && !input.Sync)),
		Report:          true,
		SyncWarning:     !shouldWrite && input.Action == "update" && !input.Sync && !familyDoc,
		FamilyPreserved: familyDoc && input.Action == "update",
	}
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
