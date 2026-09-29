package qagate

import "issueops/internal/adapter/docs"

func redactionAuditFiles(root string) []string {
	return redactionAuditFilesWithDeps(root, docsValidationDeps{listDocs: docs.ListDocs})
}
