package mcpcli

import (
	projectbootstrappddeps "issueops/internal/adapter/projectbootstrap"
	projectdocadapter "issueops/internal/adapter/projectdoc"
)

// production wiring과 같은 문서 reader를 설치한다.
func init() {
	projectbootstrappddeps.PlannedFileAction = projectdocadapter.PlannedFileAction
}
