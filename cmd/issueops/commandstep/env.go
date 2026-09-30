package commandstep

import selfverifydomain "issueops/internal/domain/selfverify"

func MergeEnvOverrides(base []string, overrides []string) []string {
	return selfverifydomain.MergeEnvOverrides(base, overrides)
}

func EnvEntryKey(entry string) (string, bool) {
	return selfverifydomain.EnvEntryKey(entry)
}
