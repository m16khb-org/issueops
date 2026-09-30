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
	if episode.Status != contract.EpisodeCompleted || episode.Host != expected.Host || episode.HostVersion != expected.HostVersion ||
		episode.RequestedModel != expected.RequestedModel || strings.TrimSpace(episode.ObservedModel) == "" || strings.TrimSpace(episode.ObservedModel) != episode.ObservedModel ||
		len(episode.ObservedModel) > 256 || strings.ContainsAny(episode.ObservedModel, "\r\n") ||
		episode.FixtureID != expected.Fixture.ID || episode.SchemaSHA256 != expected.Fixture.SchemaSHA256 ||
		episode.Profile != expected.Profile || episode.Attempt != expected.Attempt || episode.Attempt < 1 {
		return false
	}
	if episode.DurationMS <= 0 || !episode.SessionStartObserved || episode.PreToolUseObserved || episode.ExitCode != 0 || episode.AmbientToolCount != 1 || episode.CallCount != 1 {
		return false
	}
	if !ValidEvidenceID(episode.EvidenceID) || !ValidEvidenceID(episode.RawArgumentsSHA256) || !ValidEvidenceID(episode.ResponseSHA256) || episode.CanonicalArguments == nil {
		return false
	}
	if _, err := contract.ParseClassification(string(episode.Classification)); err != nil || episode.Classification == contract.Classification(contract.NoCall) || episode.Classification == contract.Classification(contract.MultipleCalls) || episode.Classification == contract.Classification(contract.InvalidJSON) {
		return false
	}
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
	validClassification := episode.Classification == contract.Classification(contract.ExactValid) || episode.Classification == contract.Classification(contract.ValidButSemanticallyDifferent)
	if validClassification && (!episode.AdvertisedValid || !episode.CanonicalValid || len(episode.Diagnostics) != 0) {
		return false
	}
	if SchemaDriftClassification(episode.Classification) && episode.CanonicalValid {
		return false
	}
	failed := episode.Classification != contract.Classification(contract.ExactValid)
	if failed {
		wantCause := "model"
		if episode.AdvertisedValid && !episode.CanonicalValid {
			wantCause = "contract_input"
		}
		if len(episode.FailureCauseEvidence) != 1 || string(episode.FailureCauseEvidence[0].Cause) != wantCause ||
			episode.FailureCauseEvidence[0].Code != string(episode.Classification) || episode.FailureCauseEvidence[0].Source != "tool_conformance" {
			return false
		}
	} else if episode.FailureCauseEvidence == nil || len(episode.FailureCauseEvidence) != 0 {
		return false
	}
	cause := failurecausedomain.Classify(failed, episode.FailureCauseEvidence)
	return episode.FailureCause == cause.Cause && episode.FailureCauseReason == cause.Reason && reflect.DeepEqual(episode.FailureCauseEvidence, cause.Evidence)
}
