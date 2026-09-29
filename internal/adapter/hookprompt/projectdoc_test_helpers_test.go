package hookprompt

import (
	projectdocadapter "issueops/internal/adapter/projectdoc"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	DiscoverProjectDocs = projectdocadapter.DiscoverProjectDocs
	FormatProjectDocCatalog = projectdocadapter.FormatProjectDocCatalog
}
