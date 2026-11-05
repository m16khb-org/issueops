package projectdocs

import docscontract "issueops/internal/contract/projectdocs"

func ReadProjectDoc(root, rel string) (docscontract.ReadResult, error) {
	return testProjectDocsService().Read(root, rel)
}
func ReviseProjectDoc(request docscontract.ReviseRequest) (docscontract.ReviseResult, error) {
	return testProjectDocsService().Revise(request)
}
func RouteProjectDocs(root, task string) (docscontract.RouteResult, error) {
	return testProjectDocsService().Route(root, task)
}
func AppendProjectDocsEntry(request docscontract.AppendRequest) (docscontract.AppendResult, error) {
	return testProjectDocsService().Append(request)
}
