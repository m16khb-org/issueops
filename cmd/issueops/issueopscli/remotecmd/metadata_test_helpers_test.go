package remotecmd

import issueopsremote "issueops/internal/domain/issueopsremote"

func normalizeRemoteCreateMetadata(labels, assignees repeatedFlag) (repeatedFlag, repeatedFlag) {
	return repeatedFlag(issueopsremote.CleanValues(labels)), repeatedFlag(issueopsremote.CleanValues(assignees))
}
