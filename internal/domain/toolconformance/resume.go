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

func ResumeHostReport(previous *contract.BenchmarkReport, host, requestedModel, hostVersion, profile string, fixtures []contract.Fixture, selected []SelectedFixturePair, targetCompleted int) (*contract.HostReport, error) {
	if previous == nil {
		return nil, nil
	}
	var matched *contract.HostReport
	for index := range previous.Hosts {
		candidate := &previous.Hosts[index]
		if candidate.Host != host {
			continue
		}
		matched = candidate
		break
	}
	if matched == nil {
		return nil, nil
	}
	if matched.Version != hostVersion || matched.RequestedModel != requestedModel {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	if matched.AttemptCount != len(matched.Cases) || matched.CompletedEpisodes != CountCompleted(matched.Cases) || matched.AttemptCount < 0 || matched.CompletedEpisodes < 0 {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	fixturesByID := make(map[string]contract.Fixture, len(fixtures))
	for _, fixture := range fixtures {
		fixturesByID[fixture.ID] = fixture
	}
	seenIdentities := map[string]bool{}
	seenEvidence := map[string]bool{}
	observedModel := ""
	for _, episode := range matched.Cases {
		identity := episode.Host + "\x00" + episode.FixtureID + "\x00" + fmt.Sprint(episode.Attempt)
		if seenIdentities[identity] {
			return nil, fmt.Errorf("duplicate_previous_episode_identity")
		}
		seenIdentities[identity] = true
		fixture, knownFixture := fixturesByID[episode.FixtureID]
		if episode.Host != host || episode.HostVersion != hostVersion || episode.RequestedModel != requestedModel || episode.Profile != profile ||
			!knownFixture || episode.SchemaSHA256 != fixture.SchemaSHA256 || episode.Attempt < 1 ||
			(episode.Status != contract.EpisodeCompleted && episode.Status != contract.EpisodeIncomplete) {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
		if episode.ObservedModel != "" {
			if strings.TrimSpace(episode.ObservedModel) != episode.ObservedModel || len(episode.ObservedModel) > 256 || strings.ContainsAny(episode.ObservedModel, "\r\n") ||
				(observedModel != "" && episode.ObservedModel != observedModel) {
				return nil, fmt.Errorf("invalid_previous_episode_evidence")
			}
			observedModel = episode.ObservedModel
		}
		if episode.Status != contract.EpisodeCompleted {
			continue
		}
		if !ValidCompletedEpisode(episode, CompletedEpisodeExpectation{
			Host: host, HostVersion: hostVersion, RequestedModel: requestedModel,
			Profile: profile, Fixture: fixture, Attempt: episode.Attempt,
		}) || episode.ObservedModel != matched.ObservedModel {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
		if seenEvidence[episode.EvidenceID] {
			return nil, fmt.Errorf("duplicate_previous_episode_evidence")
		}
		seenEvidence[episode.EvidenceID] = true
	}
	hasCompleted := matched.CompletedEpisodes > 0
	hasLiveAttempts := len(matched.Cases) > 0
	if !matched.Evidence.Installed || !matched.Evidence.PreflightReady || matched.Evidence.LiveAttempted != hasLiveAttempts ||
		matched.Evidence.LiveVerified != hasCompleted || !hasLiveAttempts || matched.ObservedModel != observedModel {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	if hasCompleted {
		if matched.Status != "supported" || strings.TrimSpace(matched.ObservedModel) == "" || matched.Evidence.StatusReason != "" {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
	} else if matched.Status != "unavailable" || matched.Evidence.StatusReason != "live_probe_incomplete" {
		return nil, fmt.Errorf("invalid_previous_episode_evidence")
	}
	for _, pair := range selected {
		if pair.Host == host && CompletedForFixture(matched.Cases, pair.Fixture.ID) > targetCompleted {
			return nil, fmt.Errorf("invalid_previous_episode_evidence")
		}
	}
	return matched, nil
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
