package issueopsapp

import (
	docsadapter "issueops/internal/adapter/docs"
	docsapp "issueops/internal/application/docs"
	"time"
)

func newDocsService() docsapp.Service {
	return docsapp.Service{Observer: docsadapter.Observer{}, Now: time.Now}
}
