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
	reviewdomain "issueops/internal/domain/issueopsreview"
)

type Reader struct {
	GitCmd    func(string, ...string) (int, string, string)
	GitCmdRaw func(string, ...string) (int, string, string)
}

func (r Reader) HasEvidence(record model.IssueOpsRecord) bool {
	worktree := strings.TrimSpace(record.WorktreePath)
	if worktree == "" || !readinesspaths.WorktreePathValid(worktree) {
		return false
	}
	if code, out, _ := r.GitCmd(worktree, "rev-parse", "--is-inside-work-tree"); code == 0 && strings.TrimSpace(out) == "true" {
		if r.gitStatusHasImplementationChange(record, worktree) {
			return true
		}
		return r.gitHeadDiffersFromBase(record, worktree)
	}
	return fileTreeHasImplementationChange(record, worktree)
}

// ChangedPaths는 현재 변경 집합의 repo-상대 경로를 정렬해 반환한다.
// ChangeFingerprint가 해시하는 것과 같은 집합이므로, fingerprint를 봉인하는
// 게이트가 "어떤 파일이 그 fingerprint에 들어 있는지"를 되물을 수 있다.
func (r Reader) ChangedPaths(record model.IssueOpsRecord) []string {
	gitRoot := r.changeGitRoot(record)
	if gitRoot == "" {
		return nil
	}
	return r.changedPathsIn(record, gitRoot)
}

func (r Reader) changeGitRoot(record model.IssueOpsRecord) string {
	gitRoot := readinesspaths.StrictGitRoot(record)
	if gitRoot == "" {
		return ""
	}
	if code, out, _ := r.GitCmd(gitRoot, "rev-parse", "--is-inside-work-tree"); code != 0 || strings.TrimSpace(out) != "true" {
		return ""
	}
	return gitRoot
}

func (r Reader) changedPathsIn(record model.IssueOpsRecord, gitRoot string) []string {
	paths := map[string]bool{}
	if base := r.DiffBaseRef(record, gitRoot); base != "" {
		_, names, _ := r.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		for _, name := range strings.Split(names, "\n") {
			if path := cleanRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	status := r.gitStatusPorcelain(gitRoot)
	for _, line := range strings.Split(status, "\n") {
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

func (r Reader) ChangeFingerprint(record model.IssueOpsRecord) string {
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

func (r Reader) ObservedPathsIn(gitRoot, base string) ([]string, bool) {
	paths := map[string]bool{}
	if base != "" {
		code, names, _ := r.GitCmd(gitRoot, "diff", "--name-only", base+"..HEAD", "--")
		if code != 0 {
			return nil, false
		}
		for _, name := range strings.Split(names, "\n") {
			if path := cleanRelativePath(name); path != "" {
				paths[path] = true
			}
		}
	}
	code, status, _ := r.GitCmdRaw(gitRoot, "status", "--porcelain=v1", "--untracked-files=all")
	if code != 0 {
		return nil, false
	}
	for _, line := range strings.Split(status, "\n") {
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

func PathMatchesPlan(record model.IssueOpsRecord, worktree, path string) bool {
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

func (r Reader) gitStatusHasImplementationChange(record model.IssueOpsRecord, worktree string) bool {
	out := r.gitStatusPorcelain(worktree)
	for _, line := range strings.Split(out, "\n") {
		path := PorcelainPath(line)
		if path == "" {
			continue
		}
		if reviewdomain.ImplementationChange(path, PathMatchesPlan(record, worktree, path)) {
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

func (r Reader) gitHeadDiffersFromBase(record model.IssueOpsRecord, worktree string) bool {
	ref := r.DiffBaseRef(record, worktree)
	if ref == "" {
		return false
	}
	_, names, _ := r.GitCmd(worktree, "diff", "--name-only", ref+"..HEAD", "--")
	for _, name := range strings.Split(names, "\n") {
		name = strings.TrimSpace(name)
		if reviewdomain.ImplementationChange(name, PathMatchesPlan(record, worktree, name)) {
			return true
		}
	}
	return false
}

func fileTreeHasImplementationChange(record model.IssueOpsRecord, worktree string) bool {
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
		if reviewdomain.ImplementationChange(path, PathMatchesPlan(record, worktree, path)) {
			found = true
		}
		return nil
	})
	return found
}

func (r Reader) DiffBaseRef(record model.IssueOpsRecord, root string) string {
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

func FingerprintSnapshot(root string, paths []string) (string, bool) {
	return fingerprintPaths(root, paths, os.ReadFile)
}

func cleanRelativePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "" || path == "." || filepath.IsAbs(path) || strings.HasPrefix(path, ".."+string(filepath.Separator)) || path == ".." {
		return ""
	}
	return filepath.ToSlash(path)
}

// ObservedChangedPaths는 ChangedPaths와 같은 관측이되 git 루트를 찾았는지를 함께
// 돌려준다. 호출자가 nil과 빈 슬라이스 구분에 기대지 않도록 명시 값으로 넘긴다.
func (r Reader) ObservedChangedPaths(record model.IssueOpsRecord) ([]string, bool) {
	gitRoot := r.changeGitRoot(record)
	if gitRoot == "" {
		return nil, false
	}
	return r.changedPathsIn(record, gitRoot), true
}
