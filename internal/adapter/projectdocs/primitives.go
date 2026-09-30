package projectdocs

import projectdocdomain "issueops/internal/domain/projectdoc"

func nonEmptyStrings(values []string) []string {
	return projectdocdomain.NonEmptyStrings(values)
}

func appendUnique(values []string, value string) []string {
	return projectdocdomain.AppendUnique(values, value)
}

func sha256Hex(content string) string {
	return projectdocdomain.SHA256Hex(content)
}

func ensureDocMetaFrontmatter(name, content string) string {
	return projectdocdomain.EnsureMetaFrontmatter(name, content)
}
