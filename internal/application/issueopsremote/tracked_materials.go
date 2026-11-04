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
// read the plan, intent, spec, architecture, and plan review after the worktree is gone.
// The sealed originals stay under artifact/, which remains ignored (#513).
const (
	trackedPlan         = "plan.md"
	trackedIntent       = "intent.md"
	trackedSpec         = "spec.md"
	trackedArchitecture = "architecture.md"
	trackedPlanReview   = "plan-review.md"
)

// trackedMaterialsDir is the worktree-relative .issueops/issues/<n>/ folder,
// or "" when the cycle has no linked issue number.
func trackedMaterialsDir(record issueops.Record) string {
	dir := ownerapp.OwnerArtifactDir(record)
	if dir == "" {
		return ""
	}
	return strings.TrimSuffix(dir, "/artifact")
}

// Write 메서드는 현재 계획과 기록에서 추적 사본을 갱신한다. 사본은 파생물이라
// 내용이 같으면 두고 다르면 덮어쓴다. 실패는 경고로 보고하며,
// 호출자의 phase 전이는 이 결과에 의존하지 않는다.
func (s TrackedMaterials) Write(record issueops.Record) issueops.TrackedMaterials {
	var out issueops.TrackedMaterials
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
	if len(out.Written) > 0 && s.Files.Ignored(root, out.Written[0]) {
		// Probe: a repository that ignores .issueops/ silently kept the
		// tracked copies out of every commit, so the PR had no plan or ledger.
		out.Warnings = append(out.Warnings, "tracked_materials_ignored: "+dir+"/ is ignored by this repository's .gitignore, so the plan, gates ledger and report will not reach the PR; ask the user whether to un-ignore it (for example .issueops/* with !.issueops/issues/ in .gitignore) or keep them local")
	}
	return out
}

type trackedMaterial struct {
	name    string
	content []byte
}

func (s TrackedMaterials) contents(record issueops.Record, root string) []trackedMaterial {
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
	if content, err := s.readSealed(sealed, record, root, "architecture"); err == nil {
		materials = append(materials, trackedMaterial{trackedArchitecture, content})
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
func (s TrackedMaterials) readSealed(sealed bool, record issueops.Record, root, name string) ([]byte, error) {
	if !sealed {
		return nil, domain.ErrSealedArtifactDirMissing
	}
	return s.Files.ReadFile(s.Files.ArtifactPath(record, root, name))
}

// trackedPlanSource is the plan file the tracked copy is made from: the
// linked plan when it lives in the sealed artifact directory, or the sealed
// plan when no plan is linked. A linked plan outside the sealed directory is
// already a tracked file, so it is not copied over the tracked plan.md.
func (s TrackedMaterials) planSource(record issueops.Record, root string) (string, bool) {
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
	if err != nil || rel == ".." || strings.HasPrefix(rel, "../") || strings.HasPrefix(rel, `..\`) {
		return "", false
	}
	return planPath, true
}

// trackedMaterialsMissing reports a cycle whose sealed plan exists but whose
// tracked plan copy does not: a cycle that entered implement before tracked
// copies existed. It reads only the local worktree.
func (s TrackedMaterials) missing(record issueops.Record) bool {
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

const trackedMaterialsMissingWarning = "tracked_materials_missing: the sealed plan has no tracked copy in .issueops/issues/<n>/; this cycle entered implement before tracked copies existed"

// materialFiles provides filesystem observations and writes; the service owns
// which materials are copied and how failures are reported.
type materialFiles interface {
	Join(...string) string
	Parent(string) string
	Resolve(string, string) string
	Relative(string, string) (string, error)
	ReadFile(string) ([]byte, error)
	WriteFile(string, []byte, fs.FileMode) error
	MkdirAll(string, fs.FileMode) error
	Stat(string) (fs.FileInfo, error)
	ArtifactPath(issueops.Record, string, string) string
	// Ignored reports whether the repository at root ignores rel.
	Ignored(root, rel string) bool
}

// TrackedMaterials 타입은 사이클 계획·의도·명세 자료를 작업 트리의 추적 폴더에 사본으로 두는 서비스다.
type TrackedMaterials struct{ Files materialFiles }

// Warning 메서드는 추적 사본이 빠졌으면 경고 문구를, 아니면 빈 문자열을 돌려준다. 로컬 작업 트리만 읽는다.
func (s TrackedMaterials) Warning(record issueops.Record) string {
	if s.missing(record) {
		return trackedMaterialsMissingWarning
	}
	return ""
}
