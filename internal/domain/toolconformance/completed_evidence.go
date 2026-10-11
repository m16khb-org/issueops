package toolconformance

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"strings"

	contract "issueops/internal/contract/toolconformance"
	failurecausedomain "issueops/internal/domain/failurecause"
)

type CompletedEpisodeExpectation struct {
	Host           string
	HostVersion    string
	RequestedModel string
	Profile        string
	Fixture        contract.Fixture
	Attempt        int
}

func ValidEvidenceID(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func DiagnosticSignature(classification contract.Classification, diagnostics []contract.Diagnostic) string {
	value := struct {
		Classification contract.Classification `json:"classification"`
		Diagnostics    []contract.Diagnostic   `json:"diagnostics"`
	}{classification, append([]contract.Diagnostic(nil), diagnostics...)}
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func ValidCompletedEpisode(episode contract.EpisodeReport, expected CompletedEpisodeExpectation) bool {
	if !completedEpisodeIdentityMatches(episode, expected) ||
		!completedEpisodeRunObserved(episode) ||
		!completedEpisodeEvidenceIDsValid(episode) ||
		!completedEpisodeClassificationAllowed(episode) ||
		!completedEpisodeDiagnosticsValid(episode) ||
		!completedEpisodeValidityConsistent(episode) {
		return false
	}
	failed := episode.Classification != contract.Classification(contract.ExactValid)
	if !completedEpisodeFailureEvidenceValid(episode, failed) {
		return false
	}
	cause := failurecausedomain.Classify(failed, episode.FailureCauseEvidence)
	return episode.FailureCause == cause.Cause && episode.FailureCauseReason == cause.Reason && reflect.DeepEqual(episode.FailureCauseEvidence, cause.Evidence)
}

func completedEpisodeIdentityMatches(episode contract.EpisodeReport, expected CompletedEpisodeExpectation) bool {
	return episode.Status == contract.EpisodeCompleted && episode.Host == expected.Host && episode.HostVersion == expected.HostVersion &&
		episode.RequestedModel == expected.RequestedModel && validObservedModel(episode.ObservedModel) &&
		episode.FixtureID == expected.Fixture.ID && episode.SchemaSHA256 == expected.Fixture.SchemaSHA256 &&
		episode.Profile == expected.Profile && episode.Attempt == expected.Attempt && episode.Attempt >= 1
}

func validObservedModel(model string) bool {
	return strings.TrimSpace(model) != "" && strings.TrimSpace(model) == model &&
		len(model) <= 256 && !strings.ContainsAny(model, "\r\n")
}

func completedEpisodeRunObserved(episode contract.EpisodeReport) bool {
	return episode.DurationMS > 0 && episode.SessionStartObserved && !episode.PreToolUseObserved &&
		episode.ExitCode == 0 && episode.AmbientToolCount == 1 && episode.CallCount == 1
}

func completedEpisodeEvidenceIDsValid(episode contract.EpisodeReport) bool {
	return ValidEvidenceID(episode.EvidenceID) && ValidEvidenceID(episode.RawArgumentsSHA256) &&
		ValidEvidenceID(episode.ResponseSHA256) && episode.CanonicalArguments != nil
}

func completedEpisodeClassificationAllowed(episode contract.EpisodeReport) bool {
	if _, err := contract.ParseClassification(string(episode.Classification)); err != nil {
		return false
	}
	return episode.Classification != contract.Classification(contract.NoCall) &&
		episode.Classification != contract.Classification(contract.MultipleCalls) &&
		episode.Classification != contract.Classification(contract.InvalidJSON)
}

func completedEpisodeDiagnosticsValid(episode contract.EpisodeReport) bool {
	orderedDiagnostics := append([]contract.Diagnostic{}, episode.Diagnostics...)
	SortDiagnostics(orderedDiagnostics)
	if !reflect.DeepEqual(orderedDiagnostics, episode.Diagnostics) || !ValidEvidenceID(episode.DiagnosticSignature) || episode.DiagnosticSignature != DiagnosticSignature(episode.Classification, episode.Diagnostics) {
		return false
	}
	for _, diagnostic := range episode.Diagnostics {
		if diagnostic.Code == "" || len(diagnostic.Path) > 512 || len(diagnostic.Code) > 128 || len(diagnostic.Expected) > 1024 || len(diagnostic.Actual) > 1024 {
			return false
		}
	}
	return true
}

func completedEpisodeValidityConsistent(episode contract.EpisodeReport) bool {
	validClassification := episode.Classification == contract.Classification(contract.ExactValid) || episode.Classification == contract.Classification(contract.ValidButSemanticallyDifferent)
	if validClassification && (!episode.AdvertisedValid || !episode.CanonicalValid || len(episode.Diagnostics) != 0) {
		return false
	}
	return !(SchemaDriftClassification(episode.Classification) && episode.CanonicalValid)
}

func completedEpisodeFailureEvidenceValid(episode contract.EpisodeReport, failed bool) bool {
	if !failed {
		return episode.FailureCauseEvidence != nil && len(episode.FailureCauseEvidence) == 0
	}
	wantCause := "model"
	if episode.AdvertisedValid && !episode.CanonicalValid {
		wantCause = "contract_input"
	}
	return len(episode.FailureCauseEvidence) == 1 && string(episode.FailureCauseEvidence[0].Cause) == wantCause &&
		episode.FailureCauseEvidence[0].Code == string(episode.Classification) && episode.FailureCauseEvidence[0].Source == "tool_conformance"
}
