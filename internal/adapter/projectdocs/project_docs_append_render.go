package projectdocs

import (
	"fmt"
	projectdocscontract "issueops/internal/contract/projectdocs"
	"strings"
	"time"
)

// renderProjectDocsAppendRecordFile renders a standalone modular record file
// with canonical frontmatter (folder-first layout, one file per record).
func renderProjectDocsAppendRecordFile(kind, name, description string, req projectdocscontract.ProjectDocsAppendRequest, now time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "---\nname: %s\ndescription: %s\n---\n\n", name, description)
	fmt.Fprintf(&b, "# %s\n\n", strings.TrimSpace(req.Title))
	fmt.Fprintf(&b, "- Date: %s\n", now.Format("2006-01-02"))
	b.WriteString(renderProjectDocsAppendFields(kind, req))
	return b.String()
}

func renderProjectDocsAppendFields(kind string, req projectdocscontract.ProjectDocsAppendRequest) string {
	var b strings.Builder
	fmt.Fprintf(&b, "- Kind: `%s`\n", kind)
	if source := strings.TrimSpace(req.Source); source != "" {
		fmt.Fprintf(&b, "- Source: %s\n", source)
	}
	fmt.Fprintf(&b, "- Summary: %s\n", strings.TrimSpace(req.Summary))
	if v := strings.TrimSpace(req.Context); v != "" {
		fmt.Fprintf(&b, "- Context: %s\n", v)
	}
	if kind == "caution" {
		if v := strings.TrimSpace(req.Resolution); v != "" {
			fmt.Fprintf(&b, "- Resolution: %s\n", v)
		}
	} else {
		if v := strings.TrimSpace(req.Decision); v != "" {
			fmt.Fprintf(&b, "- Decision: %s\n", v)
		}
		if v := strings.TrimSpace(req.Consequences); v != "" {
			fmt.Fprintf(&b, "- Consequences: %s\n", v)
		}
	}
	writeProjectDocsAppendList(&b, "- Evidence:\n", req.Evidence)
	writeProjectDocsAppendList(&b, "- Alternatives / rejected options:\n", req.Alternatives)
	return b.String()
}

// writeProjectDocsAppendList writes header and the non-blank entries; a list whose
// entries are all blank writes nothing, so records never carry empty headers.
func writeProjectDocsAppendList(b *strings.Builder, header string, entries []string) {
	wroteHeader := false
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if !wroteHeader {
			b.WriteString(header)
			wroteHeader = true
		}
		fmt.Fprintf(b, "  - %s\n", entry)
	}
}
