package staticcheck

import domain "issueops/internal/domain/apidoc"

func CheckNestDTO(file, text string) []Violation {
	return domain.CheckNestDTO(file, text)
}
func braceDepthDelta(line string) int {
	return domain.BraceDepthDelta(line)
}
