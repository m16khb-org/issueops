package apidoc

import staticcheck "issueops/internal/domain/apidoc"

func CheckNestControllerStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestController(file, text)
}
