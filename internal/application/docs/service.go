package docs

import (
	docscontract "issueops/internal/contract/docs"
	docsdomain "issueops/internal/domain/docs"
	docsport "issueops/internal/port/docs"
	"sort"
	"time"
)

type Service struct {
	Observer docsport.Observer
	Now      func() time.Time
}

func (s Service) List(root string) []string {
	candidates := s.Observer.Candidates(root, docsdomain.Roots())
	tracked, available := s.Observer.TrackedPaths(root)
	var modules []string
	if available {
		modules = s.Observer.ModuleDirectories(root)
	}
	return docsdomain.Select(candidates, tracked, available, modules)
}

func (s Service) Index(root, version string) docscontract.DocsIndexResult {
	paths := s.List(root)
	docs := make([]docscontract.DocIndexInfo, 0, len(paths))
	for _, path := range paths {
		if doc, ok := s.Observer.ReadDocument(root, path); ok {
			docs = append(docs, doc)
		}
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i].RelPath < docs[j].RelPath })
	return docscontract.DocsIndexResult{OK: true, Version: version, IssueOpsRoot: root, Docs: docs, GeneratedAt: s.Now().Format(time.RFC3339)}
}
