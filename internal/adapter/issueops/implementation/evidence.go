// Package implementation observes whether a lifecycle record's worktree holds
// implementation work: it reads git diff and status (or walks the file tree
// outside git), lists the changed paths, and fingerprints their content so the
// ai-slop-clean seal can detect later edits.
package implementation

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"issueops/internal/adapter/issueops/pathutil"
	"issueops/internal/adapter/issueops/readinesspaths"
	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsremote"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

// Reader observes a record's worktree through injected git commands: GitCmd
// returns trimmed output and GitCmdRaw returns it untrimmed, which porcelain
// status parsing needs.
type Reader struct {
	GitCmd    func(string, ...string) (int, string, string)
	GitCmdRaw func(string, ...string) (int, string, string)
}

// HasEvidence reports whether the record's worktree contains implementation
// changes, either committed since the diff base or present in the working tree.
// Changes to the linked plan or material derived from it do not count. Outside
// a git worktree it falls back to scanning the file tree.
func (r Reader) HasEvidence(record model.Record) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !readinesspaths.WorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := r.GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		// Committed changes first: the base..HEAD name diff is cheap, while the
		// untracked-inclusive status stats the whole tree (30 ms on a 20k-file
		// repo, measured). Either one is evidence, so the order only skips work.
		if r.gitHeadDiffersFromBase(record, worktree) {
			return true
		}
		return r.gitStatusHasImplementationChange(record, worktree)
	}
	return fileTreeHasImplementationChange(record, worktree)
}

// ChangedPaths 메서드는 현재 변경 집합의 repo-상대 경로를 정렬해 반환한다.
// ChangeFingerprint가 해시하는 것과 같은 집합이므로, fingerprint를 봉인하는
// 게이트가 "어떤 파일이 그 fingerprint에 들어 있는지"를 되물을 수 있다.
func (r Reader) ChangedPaths(record model.Record) []string {
	gitRoot := r.changeGitRoot(record)
	if gitRoot == "" {
		return nil
	}
	return r.changedPathsIn(record, gitRoot)
}

func (r Reader) changeGitRoot(record model.Record) string {
	gitRoot := readinesspaths.StrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := r.GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		return ""
	}
	return gitRoot
}

func (r Reader) changedPathsIn(record model.Record, gitRoot string) []string {
	paths := map[string]bool{}
	if base := r.DiffBaseRef(record, gitRoot); base != "" {
		_, names, _ := r.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		for name := range strings.SplitSeq(names, "\n") {
			if path := cleanRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	status := r.gitStatusPorcelain(gitRoot)
	for line := range strings.SplitSeq(status, "\n") {
		if path := cleanRelativePath(PorcelainPath(line)); path != "" {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	return ordered
}

// ChangeFingerprint returns a SHA-256 over the content of every changed path
// (committed since the diff base plus working-tree changes), or "" when there
// is no git root, no change, or a file cannot be read. Verification reports are
// excluded and deleted files are hashed as deleted.
func (r Reader) ChangeFingerprint(record model.Record) string {
	gitRoot := r.changeGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	ordered := r.changedPathsIn(record, gitRoot)
	if len(ordered) == 0 {
		return ""
	}
	fingerprint, ok := fingerprintPaths(gitRoot, ordered, os.ReadFile)
	if !ok {
		return ""
	}
	return fingerprint
}

// ObservedPathsIn lists the sorted repo-relative paths changed since base
// (when base is non-empty) plus the working tree under gitRoot. The boolean is
// false when either git command fails, so callers can tell failure from an
// empty change set.
func (r Reader) ObservedPathsIn(gitRoot, base string) ([]string, bool) {
	paths := map[string]bool{}
	if base != "" {
		code, names, _ := r.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		if code != 0 {
			return nil, false
		}
		for name := range strings.SplitSeq(names, "\n") {
			if path := cleanRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	code, status, _ := r.GitCmdRaw(gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	if code != 0 {
		return nil, false
	}
	for line := range strings.SplitSeq(status, "\n") {
		if path := cleanRelativePath(PorcelainPath(line)); path != "" {
			paths[path] = true
		}
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	return ordered, true
}

func fingerprintPaths(
	gitRoot string,
	ordered []string,
	readFile func(string) ([]byte, error),
) (string, bool) {
	if len(ordered) == 0 {
		return "", true
	}
	var b strings.Builder
	b.WriteString("issueops-ai-slop-clean:v1\n")
	for _, rel := range ordered {
		if issueopsremote.IsVerificationReportPath(rel) {
			// The report is written after the clean seal by verify, docs, and
			// complete; it is not part of the reviewed change.
			continue
		}
		abs := filepath.Join(gitRoot, rel)
		info, err := os.Stat(abs)
		if err != nil {
			b.WriteString(rel + "\x00deleted\n")
			continue
		}
		if info.IsDir() {
			continue
		}
		content, err := readFile(abs)
		if err != nil {
			return "", false
		}
		sum := sha256.Sum256(content)
		b.WriteString(rel + "\x00" + hex.EncodeToString(sum[:]) + "\n")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:]), true
}

// PorcelainPath extracts the path from one `git status --porcelain=v1` line,
// taking the destination of a rename and stripping quotes. It returns "" for
// lines too short to carry a path.
func PorcelainPath(line string) string {
	line = strings.TrimRight(line, "\r")
	if len(line) < 4 {
		return ""
	}
	path := strings.TrimSpace(line[3:])
	if renamed := strings.LastIndex(path, " -> "); renamed >= 0 {
		path = strings.TrimSpace(path[renamed+4:])
	}
	return strings.Trim(path, `"`)
}

// PathMatchesPlan reports whether path, resolved against worktree when
// relative, is the record's linked plan file.
func PathMatchesPlan(record model.Record, worktree, path string) bool {
	planPath := strings.TrimSpace(record.PlanPath)
	if planPath == "" || path == "" {
		return false
	}
	if !filepath.IsAbs(planPath) {
		planPath = filepath.Join(worktree, filepath.FromSlash(planPath))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(worktree, filepath.FromSlash(path))
	}
	planPath = pathutil.CleanAbsPath(planPath)
	path = pathutil.CleanAbsPath(path)
	return path == planPath
}

func (r Reader) gitStatusHasImplementationChange(record model.Record, worktree string) bool {
	out := r.gitStatusPorcelain(worktree)
	for line := range strings.SplitSeq(out, "\n") {
		path := PorcelainPath(line)
		if path == "" {
			continue
		}
		if reviewdomain.ImplementationChange(path, pathIsPlanningMaterial(record, worktree, path)) {
			return true
		}
	}
	return false
}

func (r Reader) gitStatusPorcelain(worktree string) string {
	code, out, _ := r.GitCmdRaw(worktree, "status", "--porcelain=v1", "--untracked-files=all")
	if code != 0 {
		return ""
	}
	return out
}

func (r Reader) gitHeadDiffersFromBase(record model.Record, worktree string) bool {
	ref := r.DiffBaseRef(record, worktree)
	if ref == "" {
		return false
	}
	_, names, _ := r.GitCmd(worktree, "diff", "--name-only", ref+"..HEAD", "--")
	for name := range strings.SplitSeq(names, "\n") {
		name = strings.TrimSpace(name)
		if reviewdomain.ImplementationChange(name, pathIsPlanningMaterial(record, worktree, name)) {
			return true
		}
	}
	return false
}

func fileTreeHasImplementationChange(record model.Record, worktree string) bool {
	found := false
	_ = filepath.WalkDir(worktree, func(path string, d os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if reviewdomain.ImplementationChange(path, pathIsPlanningMaterial(record, worktree, path)) {
			found = true
		}
		return nil
	})
	return found
}

// DiffBaseRef returns the first base ref from the record's branch preparation
// that resolves to a commit in root, or "" when none does.
func (r Reader) DiffBaseRef(record model.Record, root string) string {
	sha, branch := "", ""
	if record.BranchPrepare != nil {
		sha, branch = record.BranchPrepare.BaseSHA, record.BranchPrepare.BaseBranch
	}
	for _, candidate := range reviewdomain.ChangeBaseCandidates(record.BranchPrepare != nil, sha, branch) {
		args := []string{"rev-parse", "--verify"}
		if candidate.LiteralObject {
			args = append(args, "--end-of-options")
		}
		args = append(args, candidate.Ref+"^{commit}")
		if code, _, _ := r.GitCmd(root, args...); code == 0 {
			return candidate.Ref
		}
	}
	return ""
}

// FingerprintSnapshot hashes the given repo-relative paths under root the same
// way ChangeFingerprint does. The boolean is false when a file cannot be read.
func FingerprintSnapshot(root string, paths []string) (string, bool) {
	return fingerprintPaths(root, paths, os.ReadFile)
}

func cleanRelativePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) || filepath.VolumeName(path) != "" || os.IsPathSeparator(path[0]) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return ""
	}
	return filepath.ToSlash(path)
}

// ObservedChangedPaths 메서드는 ChangedPaths와 같은 관측이되 git 루트를 찾았는지를 함께
// 돌려준다. 호출자가 nil과 빈 슬라이스 구분에 기대지 않도록 명시 값으로 넘긴다.
func (r Reader) ObservedChangedPaths(record model.Record) ([]string, bool) {
	gitRoot := r.changeGitRoot(record)
	if gitRoot == "" {
		return nil, false
	}
	return r.changedPathsIn(record, gitRoot), true
}

// pathIsPlanningMaterial reports a path that is not implementation work: the
// linked plan, or a tracked material copy a phase transition derived from the
// plan and the record (#513). Such a path alone never satisfies
// implementation_changes.
func pathIsPlanningMaterial(record model.Record, worktree, path string) bool {
	if PathMatchesPlan(record, worktree, path) {
		return true
	}
	rel := path
	if filepath.IsAbs(path) {
		r, err := filepath.Rel(pathutil.CleanAbsPath(worktree), pathutil.CleanAbsPath(path))
		if err != nil {
			return false
		}
		rel = r
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	issueURL := record.IssueURL
	if strings.TrimSpace(issueURL) == "" && record.BranchPrepare != nil {
		issueURL = record.BranchPrepare.IssueURL
	}
	return issueopsremote.IsTrackedMaterialPath(issueURL, rel)
}
