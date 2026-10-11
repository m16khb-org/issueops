package quality

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const sourceGitTimeout = 10 * time.Second
const sourceGitOutputLimit = 64 * 1024 * 1024

// productionGoFiles preserves partial file results for the branch collector.
// SNR rejects any collection error instead of reporting an incomplete ratio.
func productionGoFiles(root string) ([]string, []error) {
	root, err := resolveSourceRoot(root)
	if err != nil {
		return nil, []error{err}
	}
	gitWorkspace, err := hasGitWorkspace(root)
	if err != nil {
		return nil, []error{err}
	}
	var candidates []string
	var scanErrors []error
	if gitWorkspace != "" {
		var excluded bool
		candidates, excluded, err = gitSourceCandidates(gitWorkspace, root)
		if err != nil {
			return nil, []error{err}
		}
		if excluded {
			return []string{}, nil
		}
	} else {
		if excludedSourcePath(root) {
			return []string{}, nil
		}
		candidates, scanErrors = walkSourceCandidates(root)
	}
	filter := &productionFileFilter{root: root, seen: map[string]bool{}, files: []string{}, scanErrors: scanErrors}
	for _, relative := range candidates {
		filter.add(relative)
	}
	sort.Strings(filter.files)
	return filter.files, filter.scanErrors
}

// resolveSourceRoot은 root를 절대 경로로 바꾸고 디렉터리인지 확인한다.
func resolveSourceRoot(root string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("quality source root is not a directory: %s", root)
	}
	return root, nil
}

// gitSourceCandidates는 git이 추적하거나 무시하지 않은 파일 목록을 돌려준다.
// root 자체가 제외 경로이면 excluded가 true다.
func gitSourceCandidates(gitWorkspace, root string) (candidates []string, excluded bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), sourceGitTimeout)
	defer cancel()
	output, err := sourceGitOutput(ctx, root, sourceGitOutputLimit, "ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", ".")
	if err != nil {
		return nil, false, err
	}
	scope, err := filepath.Rel(gitWorkspace, root)
	if err != nil {
		return nil, false, err
	}
	if excludedSourcePath(scope) {
		return nil, true, nil
	}
	return strings.Split(output, "\x00"), false, nil
}

// walkSourceCandidates는 git이 없는 디렉터리를 순회해 root 기준 상대 경로를 모은다.
// 순회 중 오류는 중단하지 않고 scanErrors로 모은다.
func walkSourceCandidates(root string) (candidates []string, scanErrors []error) {
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			scanErrors = append(scanErrors, err)
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if excludedSourcePath(relative) && entry.IsDir() {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			candidates = append(candidates, relative)
		}
		return nil
	})
	if err != nil {
		scanErrors = append(scanErrors, err)
	}
	return candidates, scanErrors
}

// productionFileFilter는 후보 경로에서 생산 Go 소스 파일만 골라 누적한다.
type productionFileFilter struct {
	root       string
	seen       map[string]bool
	files      []string
	scanErrors []error
}

func (f *productionFileFilter) add(relative string) {
	if relative == "" {
		return
	}
	relative = filepath.Clean(relative)
	if escapesSourceRoot(relative) {
		f.scanErrors = append(f.scanErrors, fmt.Errorf("quality source path escapes root: %s", relative))
		return
	}
	if !productionGoCandidate(relative) || f.seen[relative] {
		return
	}
	f.seen[relative] = true
	path := filepath.Join(f.root, relative)
	regular, err := regularSourcePath(f.root, relative)
	if err != nil {
		f.scanErrors = append(f.scanErrors, err)
		return
	}
	if !regular {
		return
	}
	generated, err := generatedGoSource(path)
	if err != nil {
		f.scanErrors = append(f.scanErrors, err)
		return
	}
	if generated {
		return
	}
	f.files = append(f.files, path)
}

func escapesSourceRoot(relative string) bool {
	return filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// productionGoCandidate는 제외 경로, 비Go 파일, 테스트 파일을 걸러낸다.
func productionGoCandidate(relative string) bool {
	return !excludedSourcePath(relative) && strings.HasSuffix(relative, ".go") && !strings.HasSuffix(relative, "_test.go")
}

// generatedGoSource는 파일 헤더만 파싱해 생성 코드인지 판정한다.
// PackageClauseOnly는 기존 수집기의 함수 본문 오류 처리나 SNR 줄 수 제한을 바꾸지 않는다.
func generatedGoSource(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	file, _ := parser.ParseFile(token.NewFileSet(), path, data, parser.PackageClauseOnly|parser.ParseComments)
	return file != nil && ast.IsGenerated(file), nil
}

func excludedSourcePath(relative string) bool {
	for component := range strings.SplitSeq(relative, string(filepath.Separator)) {
		switch component {
		case ".git", ".codegraph", ".issueops-runtime", ".issueops", "bin", "vendor", "node_modules", "testdata":
			return true
		}
	}
	return false
}

// A worktree .git file is also a marker. Missing or broken Git in a marked
// workspace is an error; only ordinary directories use filesystem discovery.
func hasGitWorkspace(root string) (string, error) {
	for directory := root; ; directory = filepath.Dir(directory) {
		_, err := os.Lstat(filepath.Join(directory, ".git"))
		if err == nil {
			return directory, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		if filepath.Dir(directory) == directory {
			return "", nil
		}
	}
}

func regularSourcePath(root, relative string) (bool, error) {
	path := root
	components := strings.Split(relative, string(filepath.Separator))
	for index, component := range components {
		path = filepath.Join(path, component)
		info, err := os.Lstat(path)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		} // Deleted tracked files are absent.
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return false, nil
		}
		if index == len(components)-1 {
			return info.Mode().IsRegular(), nil
		}
		if !info.IsDir() {
			return false, nil
		}
	}
	return false, nil
}

func sourceGitOutput(ctx context.Context, root string, limit int, args ...string) (string, error) {
	command := exec.CommandContext(ctx, "git", args...)
	command.Dir = root
	stdout := NewBoundedQualityBuffer(limit)
	stderr := NewBoundedQualityBuffer(64 * 1024)
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if ctx.Err() != nil {
		return "", fmt.Errorf("quality source Git: %w", ctx.Err())
	}
	if stdout.Truncated() || stderr.Truncated() {
		return "", fmt.Errorf("quality source Git output exceeds limit")
	}
	if err != nil {
		return "", fmt.Errorf("quality source Git: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
