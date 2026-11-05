package issueopscycle

import (
	model "issueops/internal/contract/issueops"
	cycledomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/stringlist"
)

type observedPRReadinessFacts struct {
	Git                cycledomain.PRGitFacts
	Artifact           observedPRFacts
	CurrentHead        string
	CurrentFingerprint string
}

func composeObservedPRReadiness(record model.Record, ready model.Readiness, facts observedPRReadinessFacts) model.Readiness {
	gitMissing, warnings := cycledomain.PRGitReadiness(record, facts.Git)
	missing := append(append([]string{}, ready.Missing...), gitMissing...)
	missing = append(missing, observedPRReadinessMissing(record, facts.Artifact)...)
	ready.Missing = stringlist.UniqueSorted(missing)
	ready.Warnings = warnings
	ready.AISlopCleanHead = record.AISlopCleanHead
	ready.CurrentHead = facts.CurrentHead
	ready.AISlopCleanFingerprint = record.AISlopCleanFingerprint
	ready.CurrentFingerprint = facts.CurrentFingerprint
	ready.Ready = len(ready.Missing) == 0
	return ready
}

func applyChildPRGate(ready model.Readiness, childMissing, childWarnings []string) model.Readiness {
	ready.Missing = stringlist.UniqueSorted(append(append([]string{}, ready.Missing...), childMissing...))
	ready.Warnings = append(ready.Warnings, childWarnings...)
	ready.Ready = len(ready.Missing) == 0
	return ready
}
