package apidoc

import staticcheck "issueops/internal/domain/apidoc"

func CheckNestDTOStatic(file, text string) []StaticViolation {
	return staticcheck.CheckNestDTO(file, text)
}
