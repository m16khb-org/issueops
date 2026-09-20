package omo

import "issueops/internal/domain/omolifecycle"

func omoLifecycleExtension(binPath string) string {
	return omolifecycle.Extension(binPath)
}

func LifecycleExtension(binPath string) string {
	return omoLifecycleExtension(binPath)
}
