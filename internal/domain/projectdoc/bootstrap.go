package projectdoc

import "strings"

// BootstrapFileKind classifies a bootstrap output file as a root document, a family module file, or the documentation manifest.
type BootstrapFileKind string

// The BootstrapFileKind values name the three kinds of file a bootstrap plan can write; each kind follows its own write and preserve policy.
const (
	BootstrapRootFile     BootstrapFileKind = "root"
	BootstrapModuleFile   BootstrapFileKind = "module"
	BootstrapManifestFile BootstrapFileKind = "manifest"
)

// BootstrapFileInput describes one planned bootstrap file: its Kind, its path Rel relative to the repository root, the planned Action ("create", "update", "unchanged"), and whether the run may Write and is in Sync mode.
type BootstrapFileInput struct {
	Kind   BootstrapFileKind
	Rel    string
	Action string
	Write  bool
	Sync   bool
}

// BootstrapFileDecision is the outcome DecideBootstrapFile reaches for one file: whether to Write it, whether an existing file is Preserved, whether it appears in the Report, whether SyncWarning should tell the user to rerun with sync, and whether FamilyPreserved marks a family document kept as is.
type BootstrapFileDecision struct {
	Write           bool
	Preserved       bool
	Report          bool
	SyncWarning     bool
	FamilyPreserved bool
}

// DecideBootstrapFile applies the bootstrap write policy to one file. Module
// and manifest files are written only on create; root files follow the family
// document, AGENTS.md, and sync rules. It is pure and never touches the
// filesystem.
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
	familyDoc := isFamilyDocRel(input.Rel)
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

func isFamilyDocRel(rel string) bool {
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
