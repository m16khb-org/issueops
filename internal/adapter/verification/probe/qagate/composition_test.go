package qagate

import (
	"issueops/internal/adapter/docs"
	docsapp "issueops/internal/application/docs"
	"time"
)

func redactionAuditFiles(root string) []string {
	return redactionAuditFilesWithDeps(root, docsValidationDeps{listDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List})
}
