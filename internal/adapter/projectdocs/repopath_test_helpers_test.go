package projectdocs

import docscontract "issueops/internal/contract/projectdocs"

func ReadProjectDoc(root, rel string) (docscontract.ProjectDocsReadResult, error) {
	return testProjectDocsService().Read(root, rel)
}
func ReviseProjectDoc(request docscontract.ProjectDocsReviseRequest) (docscontract.ProjectDocsReviseResult, error) {
	return testProjectDocsService().Revise(request)
}
func RouteProjectDocs(root, task string) (docscontract.ProjectDocsRouteResult, error) {
	return testProjectDocsService().Route(root, task)
}
func AppendProjectDocsEntry(request docscontract.ProjectDocsAppendRequest) (docscontract.ProjectDocsAppendResult, error) {
	return testProjectDocsService().Append(request)
}
