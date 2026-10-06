package projectdocs

import projectdocdomain "issueops/internal/domain/projectdoc"

func appendUnique(values []string, value string) []string {
	return projectdocdomain.AppendUnique(values, value)
}

func ensureDocMetaFrontmatter(name, content string) string {
	return projectdocdomain.EnsureMetaFrontmatter(name, content)
}
