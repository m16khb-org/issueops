package issueopsremote

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"strings"

	intentapp "issueops/internal/application/issueopsintent"
	ownerapp "issueops/internal/application/issueopsowner"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/domain/issueopsintent"
)

// Tracked copies of the implementation materials live next to gates.md in
// .issueops/issues/<n>/ and are committed with the change, so reviewers can
// read the plan, intent, spec, and plan review after the worktree is gone.
// The sealed originals stay under artifact/, which remains ignored (#513).
const (
	trackedPlan       = "plan.md"
	trackedIntent     = "intent.md"
	trackedSpec       = "spec.md"
	trackedPlanReview = "plan-review.md"
)

// trackedMaterialsDir is the worktree-relative .issueops/issues/<n>/ folder,
// or "" when the cycle has no linked issue number.
func trackedMaterialsDir(record issueops.IssueOpsRecord) string {
	dir := ownerapp.OwnerArtifactDir(record)
	if dir == "" {
		return ""
	}
	return strings.TrimSuffix(dir, "/artifact")
}

// writeTrackedMaterials refreshes the tracked copies from the current plan
// and record. Copies are derived: identical content is left alone, different
// content is overwritten. Failures are reported as warnings; the caller's
// phase transition does not depend on them.
func (s TrackedMaterials) Write(record issueops.IssueOpsRecord) issueops.IssueOpsTrackedMaterials {
	var out issueops.IssueOpsTrackedMaterials
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return out
	}
	dir := trackedMaterialsDir(record)
	if dir == "" {
		out.Warnings = append(out.Warnings, "tracked_materials_skipped: the cycle has no linked issue number")
		return out
	}
	root := record.Execution.Workspace.Root
	for _, material := range s.contents(record, root) {
		path := s.Files.Join(root, dir, material.name)
		existing, err := s.Files.ReadFile(path)
		if err == nil && bytes.Equal(existing, material.content) {
			continue
		}
		if err := s.Files.MkdirAll(s.Files.Parent(path), 0o755); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("tracked_materials_write_failed:%s: %v", material.name, err))
			continue
		}
		if err := s.Files.WriteFile(path, material.content, 0o644); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("tracked_materials_write_failed:%s: %v", material.name, err))
			continue
		}
		out.Written = append(out.Written, dir+"/"+material.name)
	}
	return out
}

type trackedMaterial struct {
	name    string
	content []byte
}

func (s TrackedMaterials) contents(record issueops.IssueOpsRecord, root string) []trackedMaterial {
	var materials []trackedMaterial
	if planPath, ok := s.planSource(record, root); ok {
		if content, err := s.Files.ReadFile(planPath); err == nil {
			materials = append(materials, trackedMaterial{trackedPlan, content})
		}
	}
	sealed := domain.RequireSealedArtifactDir(record) == nil
	if content, err := s.readSealed(sealed, record, root, "intent"); err == nil {
		materials = append(materials, trackedMaterial{trackedIntent, content})
	} else if record.Intent != nil {
		materials = append(materials, trackedMaterial{trackedIntent, []byte(issueopsintent.Render(intentapp.IntentDocument(record)))})
	}
	if content, err := s.readSealed(sealed, record, root, "spec"); err == nil {
		materials = append(materials, trackedMaterial{trackedSpec, content})
	}
	if review := record.DevilsAdvocateReview; review != nil {
		materials = append(materials, trackedMaterial{trackedPlanReview, []byte(domain.RenderTrackedPlanReview(review))})
	}
	for i := range materials {
		materials[i].content = domain.NormalizePublicMaterialPaths(materials[i].content, record.Repo, root)
	}
	return materials
}

// readSealed reads a sealed artifact only when the record names its sealed
// directory; a record without one has no sealed files to copy.
func (s TrackedMaterials) readSealed(sealed bool, record issueops.IssueOpsRecord, root, name string) ([]byte, error) {
	if !sealed {
		return nil, domain.ErrSealedArtifactDirMissing
	}
	return s.Files.ReadFile(s.Files.ArtifactPath(record, root, name))
}

// trackedPlanSource is the plan file the tracked copy is made from: the
// linked plan when it lives in the sealed artifact directory, or the sealed
// plan when no plan is linked. A linked plan outside the sealed directory is
// already a tracked file, so it is not copied over the tracked plan.md.
func (s TrackedMaterials) planSource(record issueops.IssueOpsRecord, root string) (string, bool) {
	if domain.RequireSealedArtifactDir(record) != nil {
		return "", false
	}
	sealedDir := s.Files.Parent(s.Files.ArtifactPath(record, root, "plan"))
	planPath := strings.TrimSpace(record.PlanPath)
	if planPath == "" {
		return s.Files.Join(sealedDir, "plan.md"), true
	}
	planPath = s.Files.Resolve(root, planPath)
	rel, err := s.Files.Relative(sealedDir, planPath)
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") {
		return "", false
	}
	return planPath, true
}

// trackedMaterialsMissing reports a cycle whose sealed plan exists but whose
// tracked plan copy does not: a cycle that entered implement before tracked
// copies existed. It reads only the local worktree.
func (s TrackedMaterials) Missing(record issueops.IssueOpsRecord) bool {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return false
	}
	dir := trackedMaterialsDir(record)
	root := record.Execution.Workspace.Root
	source, ok := s.planSource(record, root)
	if dir == "" || !ok {
		return false
	}
	if _, err := s.Files.Stat(source); err != nil {
		return false
	}
	_, err := s.Files.Stat(s.Files.Join(root, dir, trackedPlan))
	return errors.Is(err, fs.ErrNotExist)
}

const TrackedMaterialsMissingWarning = "tracked_materials_missing: the sealed plan has no tracked copy in .issueops/issues/<n>/; this cycle entered implement before tracked copies existed"

// MaterialFiles provides filesystem observations and writes; the service owns
// which materials are copied and how failures are reported.
type MaterialFiles interface {
	Join(...string) string
	Parent(string) string
	Resolve(string, string) string
	Relative(string, string) (string, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	Stat(string) (fs.FileInfo, error)
	ArtifactPath(issueops.IssueOpsRecord, string, string) string
}
type TrackedMaterials struct{ Files MaterialFiles }

func (s TrackedMaterials) Warning(record issueops.IssueOpsRecord) string {
	if s.Missing(record) {
		return TrackedMaterialsMissingWarning
	}
	return ""
}
