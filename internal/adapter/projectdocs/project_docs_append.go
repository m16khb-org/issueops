package projectdocs

import (
	projectdocsapp "issueops/internal/application/projectdocs"
	projectdocscontract "issueops/internal/contract/projectdocs"
)

func AppendProjectDocsEntry(request projectdocscontract.ProjectDocsAppendRequest) (projectdocscontract.ProjectDocsAppendResult, error) {
	root, err := NormalizeRepoRoot(request.RepoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	return projectdocsapp.Append(root, request, appendFileEffects{})
}
