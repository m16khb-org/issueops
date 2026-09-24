package issueops

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/issueops/intentdesign"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/artifactreadability"
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
	dir := issueArtifactDirFor(record)
	if dir == "" {
		return ""
	}
	return filepath.Dir(filepath.FromSlash(dir))
}

// writeTrackedMaterials refreshes the tracked copies from the current plan
// and record. Copies are derived: identical content is left alone, different
// content is overwritten. Failures are reported as warnings; the caller's
// phase transition does not depend on them.
func writeTrackedMaterials(record issueops.IssueOpsRecord) issueops.IssueOpsTrackedMaterials {
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
	for _, material := range trackedMaterialContents(record, root) {
		path := filepath.Join(root, dir, material.name)
		existing, err := os.ReadFile(path)
		if err == nil && bytes.Equal(existing, material.content) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("tracked_materials_write_failed:%s: %v", material.name, err))
			continue
		}
		if err := os.WriteFile(path, material.content, 0o644); err != nil {
			out.Warnings = append(out.Warnings, fmt.Sprintf("tracked_materials_write_failed:%s: %v", material.name, err))
			continue
		}
		out.Written = append(out.Written, filepath.ToSlash(filepath.Join(dir, material.name)))
	}
	return out
}

type trackedMaterial struct {
	name    string
	content []byte
}

func trackedMaterialContents(record issueops.IssueOpsRecord, root string) []trackedMaterial {
	var materials []trackedMaterial
	if planPath, ok := trackedPlanSource(record, root); ok {
		if content, err := os.ReadFile(planPath); err == nil {
			materials = append(materials, trackedMaterial{trackedPlan, content})
		}
	}
	if content, err := os.ReadFile(sealedArtifactPath(record, root, "intent")); err == nil {
		materials = append(materials, trackedMaterial{trackedIntent, content})
	} else if record.Intent != nil {
		materials = append(materials, trackedMaterial{trackedIntent, []byte(issueopsintent.Render(intentdesign.IntentDocument(record)))})
	}
	if content, err := os.ReadFile(sealedArtifactPath(record, root, "spec")); err == nil {
		materials = append(materials, trackedMaterial{trackedSpec, content})
	}
	if review := record.DevilsAdvocateReview; review != nil {
		materials = append(materials, trackedMaterial{trackedPlanReview, []byte(renderTrackedPlanReview(review))})
	}
	return materials
}

// trackedPlanSource is the plan file the tracked copy is made from: the
// linked plan when it lives in the sealed artifact directory, or the sealed
// plan when no plan is linked. A linked plan outside the sealed directory is
// already a tracked file, so it is not copied over the tracked plan.md.
func trackedPlanSource(record issueops.IssueOpsRecord, root string) (string, bool) {
	sealedDir := filepath.Join(root, filepath.FromSlash(sealedArtifactDir(record)))
	planPath := strings.TrimSpace(record.PlanPath)
	if planPath == "" {
		return filepath.Join(sealedDir, "plan.md"), true
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(root, planPath)
	}
	rel, err := filepath.Rel(sealedDir, filepath.Clean(planPath))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return planPath, true
}

// trackedMaterialsMissing reports a cycle whose sealed plan exists but whose
// tracked plan copy does not: a cycle that entered implement before tracked
// copies existed. It reads only the local worktree.
func trackedMaterialsMissing(record issueops.IssueOpsRecord) bool {
	if record.Execution == nil || strings.TrimSpace(record.Execution.Workspace.Root) == "" {
		return false
	}
	dir := trackedMaterialsDir(record)
	root := record.Execution.Workspace.Root
	source, ok := trackedPlanSource(record, root)
	if dir == "" || !ok {
		return false
	}
	if _, err := os.Stat(source); err != nil {
		return false
	}
	_, err := os.Stat(filepath.Join(root, dir, trackedPlan))
	return os.IsNotExist(err)
}

const trackedMaterialsMissingWarning = "tracked_materials_missing: the sealed plan has no tracked copy in .issueops/issues/<n>/; this cycle entered implement before tracked copies existed"

// renderTrackedPlanReview writes every plan-review round with its findings
// for readers who want more than the issue's one-line summary. Hashes and
// local paths in findings are masked the same way the issue region masks them.
func renderTrackedPlanReview(review *issueops.IssueOpsDevilsAdvocateReview) string {
	var b strings.Builder
	b.WriteString("# 계획 검토 기록\n")
	rounds := append(append([]issueops.IssueOpsDevilsAdvocateRound{}, review.History...), issueops.IssueOpsDevilsAdvocateRound{
		Verdict: review.Verdict, Findings: review.Findings, Waived: review.Waived, WaiverRationale: review.WaiverRationale,
	})
	for i, round := range rounds {
		fmt.Fprintf(&b, "\n## %d차: %s\n", i+1, artifactreadability.PlanReviewVerdictLabel(round.Verdict))
		if round.Waived {
			fmt.Fprintf(&b, "\n생략: %s\n", artifactreadability.MaskHarnessValues(strings.TrimSpace(round.WaiverRationale)))
		}
		if len(round.Findings) == 0 {
			continue
		}
		b.WriteString("\n")
		for _, finding := range round.Findings {
			if finding = strings.TrimSpace(finding); finding != "" {
				fmt.Fprintf(&b, "- %s\n", artifactreadability.MaskHarnessValues(finding))
			}
		}
	}
	return b.String()
}
