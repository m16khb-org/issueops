package issueopsremote

import (
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

// memoryMaterialFiles is an in-memory worktree; paths are slash-separated.
type memoryMaterialFiles struct {
	files    map[string][]byte
	writeErr map[string]error
	writes   []string
}

func newMemoryMaterialFiles() *memoryMaterialFiles {
	return &memoryMaterialFiles{files: map[string][]byte{}, writeErr: map[string]error{}}
}

func (f *memoryMaterialFiles) Join(parts ...string) string { return path.Join(parts...) }
func (f *memoryMaterialFiles) Parent(p string) string      { return path.Dir(p) }
func (f *memoryMaterialFiles) Resolve(root, p string) string {
	if path.IsAbs(p) {
		return path.Clean(p)
	}
	return path.Join(root, p)
}
func (f *memoryMaterialFiles) Relative(base, target string) (string, error) {
	if target == base || strings.HasPrefix(target, base+"/") {
		return strings.TrimPrefix(strings.TrimPrefix(target, base), "/"), nil
	}
	return "../" + target, nil
}
func (f *memoryMaterialFiles) ReadFile(p string) ([]byte, error) {
	if b, ok := f.files[p]; ok {
		return b, nil
	}
	return nil, fs.ErrNotExist
}
func (f *memoryMaterialFiles) WriteFile(p string, b []byte, _ fs.FileMode) error {
	if err := f.writeErr[path.Base(p)]; err != nil {
		return err
	}
	f.files[p] = b
	f.writes = append(f.writes, p)
	return nil
}
func (f *memoryMaterialFiles) MkdirAll(string, fs.FileMode) error { return nil }
func (f *memoryMaterialFiles) Stat(p string) (fs.FileInfo, error) {
	if _, ok := f.files[p]; ok {
		return nil, nil
	}
	return nil, fs.ErrNotExist
}
func (f *memoryMaterialFiles) ArtifactPath(record model.IssueOpsRecord, root, name string) string {
	return path.Join(root, record.Execution.Workspace.ArtifactDir, name+".md")
}

const (
	trackedRoot   = "/wt"
	trackedSealed = "/wt/.issueops/issues/7/artifact"
)

func trackedRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		IssueURL: "https://github.com/acme/app/issues/7",
		Execution: &model.Execution{Workspace: model.Workspace{
			Root: trackedRoot, ArtifactDir: ".issueops/issues/7/artifact",
		}},
	}
}

func TestTrackedMaterialsWriteCopiesSealedFilesAndSkipsIdenticalContent(t *testing.T) {
	files := newMemoryMaterialFiles()
	files.files[trackedSealed+"/plan.md"] = []byte("# plan\n")
	files.files[trackedSealed+"/spec.md"] = []byte("# spec\n")
	svc := TrackedMaterials{Files: files}

	got := svc.Write(trackedRecord())
	want := []string{".issueops/issues/7/plan.md", ".issueops/issues/7/spec.md"}
	if strings.Join(got.Written, ",") != strings.Join(want, ",") || len(got.Warnings) != 0 {
		t.Fatalf("got=%+v", got)
	}
	if string(files.files["/wt/.issueops/issues/7/spec.md"]) != "# spec\n" {
		t.Fatalf("spec not copied: %v", files.files)
	}

	files.writes = nil
	if again := svc.Write(trackedRecord()); len(again.Written) != 0 || len(files.writes) != 0 {
		t.Fatalf("identical copies must not be rewritten: %+v writes=%v", again, files.writes)
	}

	files.files[trackedSealed+"/plan.md"] = []byte("# plan v2\n")
	if changed := svc.Write(trackedRecord()); len(changed.Written) != 1 || changed.Written[0] != ".issueops/issues/7/plan.md" {
		t.Fatalf("changed sealed plan must overwrite only its copy: %+v", changed)
	}
}

func TestTrackedMaterialsWriteNoopWithoutWorkspaceAndWarnsWithoutIssueNumber(t *testing.T) {
	svc := TrackedMaterials{Files: newMemoryMaterialFiles()}
	if got := svc.Write(model.IssueOpsRecord{}); len(got.Written)+len(got.Warnings) != 0 {
		t.Fatalf("no execution: %+v", got)
	}
	record := trackedRecord()
	record.IssueURL = ""
	got := svc.Write(record)
	if len(got.Warnings) != 1 || !strings.HasPrefix(got.Warnings[0], "tracked_materials_skipped:") || len(got.Written) != 0 {
		t.Fatalf("got=%+v", got)
	}
}

func TestTrackedMaterialsWriteReportsFailuresAsWarningsAndContinues(t *testing.T) {
	files := newMemoryMaterialFiles()
	files.files[trackedSealed+"/plan.md"] = []byte("plan")
	files.files[trackedSealed+"/spec.md"] = []byte("spec")
	files.writeErr["plan.md"] = errors.New("disk full")
	got := TrackedMaterials{Files: files}.Write(trackedRecord())
	if len(got.Warnings) != 1 || got.Warnings[0] != "tracked_materials_write_failed:plan.md: disk full" {
		t.Fatalf("warnings=%v", got.Warnings)
	}
	if len(got.Written) != 1 || got.Written[0] != ".issueops/issues/7/spec.md" {
		t.Fatalf("a failed copy must not block the others: %+v", got)
	}
}

func TestTrackedMaterialsWriteRendersPlanReviewAndMasksLocalPaths(t *testing.T) {
	files := newMemoryMaterialFiles()
	files.files[trackedSealed+"/plan.md"] = []byte("see /wt/src/main.go\n")
	record := trackedRecord()
	record.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{Verdict: "pass", Findings: []string{"  looks fine  "}}
	got := TrackedMaterials{Files: files}.Write(record)
	if len(got.Written) != 2 {
		t.Fatalf("got=%+v", got)
	}
	plan := string(files.files["/wt/.issueops/issues/7/plan.md"])
	if strings.Contains(plan, "/wt/") {
		t.Fatalf("worktree absolute path leaked into tracked copy: %q", plan)
	}
	review := string(files.files["/wt/.issueops/issues/7/plan-review.md"])
	if !strings.HasPrefix(review, "# 계획 검토 기록\n") || !strings.Contains(review, "- looks fine\n") {
		t.Fatalf("review=%q", review)
	}
}

func TestTrackedMaterialsPlanSourceHonorsLinkedPlanLocation(t *testing.T) {
	files := newMemoryMaterialFiles()
	files.files[trackedSealed+"/custom-plan.md"] = []byte("linked")
	files.files["/wt/docs/plan.md"] = []byte("tracked elsewhere")
	svc := TrackedMaterials{Files: files}

	record := trackedRecord()
	record.PlanPath = ".issueops/issues/7/artifact/custom-plan.md"
	got := svc.Write(record)
	if len(got.Written) != 1 || string(files.files["/wt/.issueops/issues/7/plan.md"]) != "linked" {
		t.Fatalf("linked sealed plan must be copied: %+v", got)
	}

	record.PlanPath = "docs/plan.md"
	files2 := newMemoryMaterialFiles()
	files2.files["/wt/docs/plan.md"] = []byte("tracked elsewhere")
	if got := (TrackedMaterials{Files: files2}).Write(record); len(got.Written) != 0 {
		t.Fatalf("a plan outside the sealed dir is already tracked and must not be copied: %+v", got)
	}
}

func TestTrackedMaterialsMissingDetectsLegacyCycles(t *testing.T) {
	files := newMemoryMaterialFiles()
	svc := TrackedMaterials{Files: files}
	record := trackedRecord()
	if svc.Missing(record) || svc.Warning(record) != "" {
		t.Fatal("no sealed plan means nothing is missing")
	}
	files.files[trackedSealed+"/plan.md"] = []byte("plan")
	if !svc.Missing(record) || svc.Warning(record) != TrackedMaterialsMissingWarning {
		t.Fatal("sealed plan without tracked copy must be reported")
	}
	files.files["/wt/.issueops/issues/7/plan.md"] = []byte("plan")
	if svc.Missing(record) || svc.Warning(record) != "" {
		t.Fatal("tracked copy present must clear the warning")
	}
	if svc.Missing(model.IssueOpsRecord{}) {
		t.Fatal("record without workspace cannot be missing copies")
	}
}
