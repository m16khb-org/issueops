package staticcheck

import domain "issueops/internal/domain/apidoc"

func CheckNestController(file, text string) []Violation {
	return domain.CheckNestController(file, text)
}
func hasNestResponseStatus(block string, status int) bool {
	return domain.HasNestResponseStatus(block, status)
}
func isNestPrivateRoute(block, fileText string) bool {
	return domain.IsNestPrivateRoute(block, fileText)
}
