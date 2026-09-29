package nativeintegration

import (
	"os"
)

type nativeIntegrationValidationDeps struct {
	Validator
	userHomeDir             func() (string, error)
	exists                  func(string) bool
	readFile                func(string) ([]byte, error)
	duplicateWarningFixture func() string
}

func (deps nativeIntegrationValidationDeps) withDefaults() nativeIntegrationValidationDeps {
	if deps.userHomeDir == nil {
		deps.userHomeDir = os.UserHomeDir
	}
	if deps.exists == nil {
		deps.exists = exists
	}
	if deps.readFile == nil {
		deps.readFile = os.ReadFile
	}
	if deps.duplicateWarningFixture == nil {
		deps.duplicateWarningFixture = claudeMCPDuplicateWarningFixture
	}
	return deps
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
