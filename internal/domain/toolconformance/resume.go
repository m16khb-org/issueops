package toolconformance

import (
	"fmt"
	contract "issueops/internal/contract/toolconformance"
	"strings"
)

type SelectedFixturePair struct {
	Host    string
	Fixture contract.Fixture
}

type resumeIdentity struct {
	host, hostVersion, requestedModel, profile string
	fixturesByID                               map[string]contract.Fixture
}

func errInvalidPreviousEvidence() error {
	return fmt.Errorf("invalid_previous_episode_evidence")
}

func ResumeHostReport(previous *contract.BenchmarkReport, host, requestedModel, hostVersion, profile string, fixtures []contract.Fixture, selected []SelectedFixturePair, targetCompleted int) (*contract.HostReport, error) {
	if previous == nil {
		return nil, nil
	}
	matched := findHostReport(previous, host)
	if matched == nil {
		return nil, nil
	}
	if matched.Version != hostVersion || matched.RequestedModel != requestedModel {
		return nil, errInvalidPreviousEvidence()
	}
	if matched.AttemptCount != len(matched.Cases) || matched.CompletedEpisodes != CountCompleted(matched.Cases) || matched.AttemptCount < 0 || matched.CompletedEpisodes < 0 {
		return nil, errInvalidPreviousEvidence()
	}
	fixturesByID := make(map[string]contract.Fixture, len(fixtures))
	for _, fixture := range fixtures {
		fixturesByID[fixture.ID] = fixture
	}
	identity := resumeIdentity{host: host, hostVersion: hostVersion, requestedModel: requestedModel, profile: profile, fixturesByID: fixturesByID}
	observedModel, err := validatePreviousEpisodes(matched, identity)
	if err != nil {
		return nil, err
	}
	if err := validatePreviousHostStatus(matched, observedModel); err != nil {
		return nil, err
	}
	for _, pair := range selected {
		if pair.Host == host && CompletedForFixture(matched.Cases, pair.Fixture.ID) > targetCompleted {
			return nil, errInvalidPreviousEvidence()
		}
	}
	return matched, nil
}

func findHostReport(previous *contract.BenchmarkReport, host string) *contract.HostReport {
	for index := range previous.Hosts {
		if previous.Hosts[index].Host == host {
			return &previous.Hosts[index]
		}
	}
	return nil
}

// validatePreviousEpisodes checks every recorded episode and returns the observed model they share.
func validatePreviousEpisodes(matched *contract.HostReport, id resumeIdentity) (string, error) {
	seenIdentities := map[string]bool{}
	seenEvidence := map[string]bool{}
	observedModel := ""
	for _, episode := range matched.Cases {
		key := episode.Host + "\x00" + episode.FixtureID + "\x00" + fmt.Sprint(episode.Attempt)
		if seenIdentities[key] {
			return "", fmt.Errorf("duplicate_previous_episode_identity")
		}
		seenIdentities[key] = true
		fixture, knownFixture := id.fixturesByID[episode.FixtureID]
		if !previousEpisodeMatchesRun(episode, id, fixture, knownFixture) {
			return "", errInvalidPreviousEvidence()
		}
		if episode.ObservedModel != "" {
			if !consistentObservedModel(episode.ObservedModel, observedModel) {
				return "", errInvalidPreviousEvidence()
			}
			observedModel = episode.ObservedModel
		}
		if episode.Status != contract.EpisodeCompleted {
			continue
		}
		if !ValidCompletedEpisode(episode, CompletedEpisodeExpectation{
			Host: id.host, HostVersion: id.hostVersion, RequestedModel: id.requestedModel,
			Profile: id.profile, Fixture: fixture, Attempt: episode.Attempt,
		}) || episode.ObservedModel != matched.ObservedModel {
			return "", errInvalidPreviousEvidence()
		}
		if seenEvidence[episode.EvidenceID] {
			return "", fmt.Errorf("duplicate_previous_episode_evidence")
		}
		seenEvidence[episode.EvidenceID] = true
	}
	return observedModel, nil
}

func previousEpisodeMatchesRun(episode contract.EpisodeReport, id resumeIdentity, fixture contract.Fixture, knownFixture bool) bool {
	if episode.Host != id.host || episode.HostVersion != id.hostVersion || episode.RequestedModel != id.requestedModel || episode.Profile != id.profile {
		return false
	}
	if !knownFixture || episode.SchemaSHA256 != fixture.SchemaSHA256 || episode.Attempt < 1 {
		return false
	}
	return episode.Status == contract.EpisodeCompleted || episode.Status == contract.EpisodeIncomplete
}

func consistentObservedModel(model, previouslyObserved string) bool {
	if strings.TrimSpace(model) != model || len(model) > 256 || strings.ContainsAny(model, "\r\n") {
		return false
	}
	return previouslyObserved == "" || model == previouslyObserved
}

func validatePreviousHostStatus(matched *contract.HostReport, observedModel string) error {
	hasCompleted := matched.CompletedEpisodes > 0
	hasLiveAttempts := len(matched.Cases) > 0
	if !matched.Evidence.Installed || !matched.Evidence.PreflightReady || matched.Evidence.LiveAttempted != hasLiveAttempts ||
		matched.Evidence.LiveVerified != hasCompleted || !hasLiveAttempts || matched.ObservedModel != observedModel {
		return errInvalidPreviousEvidence()
	}
	if hasCompleted {
		if matched.Status != "supported" || strings.TrimSpace(matched.ObservedModel) == "" || matched.Evidence.StatusReason != "" {
			return errInvalidPreviousEvidence()
		}
	} else if matched.Status != "unavailable" || matched.Evidence.StatusReason != "live_probe_incomplete" {
		return errInvalidPreviousEvidence()
	}
	return nil
}

func ValidatePreviousSelection(previous *contract.BenchmarkReport, selected []SelectedFixturePair) error {
	if previous == nil {
		return nil
	}
	seenHosts := make(map[string]bool, len(previous.Hosts))
	for _, hostReport := range previous.Hosts {
		if hostReport.Host == "" || seenHosts[hostReport.Host] {
			return fmt.Errorf("invalid_previous_report_identity")
		}
		seenHosts[hostReport.Host] = true
	}
	for _, hostReport := range previous.Hosts {
		selectedHost := false
		for _, pair := range selected {
			if pair.Host == hostReport.Host {
				selectedHost = true
				break
			}
		}
		if !selectedHost {
			return fmt.Errorf("invalid_previous_episode_selection")
		}
		for _, episode := range hostReport.Cases {
			if _, selectedPair := SelectedFixtureForPair(selected, hostReport.Host, episode.FixtureID); !selectedPair {
				return fmt.Errorf("invalid_previous_episode_selection")
			}
		}
	}
	return nil
}

func SelectedFixtureForPair(pairs []SelectedFixturePair, host, fixture string) (contract.Fixture, bool) {
	for _, pair := range pairs {
		if pair.Host == host && pair.Fixture.ID == fixture {
			return pair.Fixture, true
		}
	}
	return contract.Fixture{}, false
}

func CompletedForFixture(episodes []contract.EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture && episode.Status == "completed" {
			count++
		}
	}
	return count
}

func AttemptsForFixture(episodes []contract.EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture {
			count++
		}
	}
	return count
}

func CountCompleted(episodes []contract.EpisodeReport) int {
	count := 0
	for _, episode := range episodes {
		if episode.Status == "completed" {
			count++
		}
	}
	return count
}
