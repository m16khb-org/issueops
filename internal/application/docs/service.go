package docs

import (
	docscontract "issueops/internal/contract/docs"
	docsdomain "issueops/internal/domain/docs"
	docsport "issueops/internal/port/docs"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

type Service struct {
	Observer docsport.Observer
	Now      func() time.Time
}

// List overlaps the Git tracked-path query with the filesystem walk; the
// Observer must be safe for concurrent use.
func (s Service) List(root string) []string {
	_, selected := s.startList(root)
	return selected()
}

// startList walks the doc roots while Git resolves the tracked set in the
// background; selected waits for Git and returns the chosen candidate paths.
func (s Service) startList(root string) ([]docscontract.Candidate, func() []string) {
	var (
		tracked   map[string]bool
		available bool
		modules   []string
		wg        sync.WaitGroup
	)
	wg.Go(func() {
		tracked, available = s.Observer.TrackedPaths(root)
		if available {
			modules = s.Observer.ModuleDirectories(root)
		}
	})
	candidates := s.Observer.Candidates(root, docsdomain.Roots())
	return candidates, func() []string {
		wg.Wait()
		return docsdomain.Select(candidates, tracked, available, modules)
	}
}

func (s Service) Index(root, version string) docscontract.DocsIndexResult {
	// Selection only drops eligible candidates, so they are read while Git is
	// still resolving which of them are selected.
	candidates, selected := s.startList(root)
	eligible := docsdomain.Eligible(candidates)
	read := make([]docscontract.DocIndexInfo, len(eligible))
	found := make([]bool, len(eligible))
	var (
		next atomic.Int64
		wg   sync.WaitGroup
	)
	// Reads overlap the Git wait, so a few workers hide them; more only add
	// syscall contention CPU.
	for range min(runtime.GOMAXPROCS(0), 4, len(eligible)) {
		wg.Go(func() {
			for i := int(next.Add(1) - 1); i < len(eligible); i = int(next.Add(1) - 1) {
				read[i], found[i] = s.Observer.ReadDocument(root, eligible[i].Path)
			}
		})
	}
	byPath := make(map[string]int, len(eligible))
	for i, candidate := range eligible {
		byPath[candidate.Path] = i
	}
	paths := selected()
	wg.Wait()
	docs := make([]docscontract.DocIndexInfo, 0, len(paths))
	for _, path := range paths {
		if i, ok := byPath[path]; ok && found[i] {
			docs = append(docs, read[i])
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
	return docscontract.DocsIndexResult{OK: true, Version: version, IssueOpsRoot: root, Docs: docs, GeneratedAt: s.Now().Format(time.RFC3339)}
}
