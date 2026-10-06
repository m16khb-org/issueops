package qagate

import "path/filepath"

func validateMermaidDocsWithDeps(root string, deps docsValidationDeps) []string {
	deps = deps.withDefaults()
	errs := []string{}
	for _, path := range deps.listDocs(root) {
		b, err := deps.readFile(path)
		if err != nil {
			errs = append(errs, "read mermaid doc "+path+": "+err.Error())
			continue
		}
		rel, err := deps.rel(root, path)
		if err != nil {
			rel = path
		}
		errs = append(errs, LintMermaidBlocks(filepath.ToSlash(rel), string(b))...)
	}
	return errs
}
