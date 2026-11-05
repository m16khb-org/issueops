package toolconformance

import (
	"fmt"
	contract "issueops/internal/contract/toolconformance"
	"strings"
)

// SelectedFixturePair is a host together with a fixture selected to run against it.
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

// ResumeHostReport validates the previous report's entry for host and returns it so a benchmark run can continue from it.
// It returns nil without error when there is no previous report or no entry for host,
// and an invalid-previous-evidence error when the entry's version, model, counts, episodes, status, or selection do not fit the current run.
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
	if !hostCountsConsistent(matched) {
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
	if selectionExceedsTarget(matched, host, selected, targetCompleted) {
		return nil, errInvalidPreviousEvidence()
	}
	return matched, nil
}

// hostCountsConsistent reports whether the recorded attempt and completed counts match the recorded episodes.
func hostCountsConsistent(matched *contract.HostReport) bool {
	if matched.AttemptCount != len(matched.Cases) || matched.CompletedEpisodes != CountCompleted(matched.Cases) {
		return false
	}
	return matched.AttemptCount >= 0 && matched.CompletedEpisodes >= 0
}

// selectionExceedsTarget reports whether a selected fixture of the host already has more completed episodes than the target.
func selectionExceedsTarget(matched *contract.HostReport, host string, selected []SelectedFixturePair, targetCompleted int) bool {
	for _, pair := range selected {
		if pair.Host == host && CompletedForFixture(matched.Cases, pair.Fixture.ID) > targetCompleted {
			return true
		}
	}
	return false
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
	if !previousHostStatusConsistent(matched, hasCompleted) {
		return errInvalidPreviousEvidence()
	}
	return nil
}

func previousHostStatusConsistent(matched *contract.HostReport, hasCompleted bool) bool {
	if hasCompleted {
		return matched.Status == "supported" && strings.TrimSpace(matched.ObservedModel) != "" && matched.Evidence.StatusReason == ""
	}
	return matched.Status == "unavailable" && matched.Evidence.StatusReason == "live_probe_incomplete"
}

// ValidatePreviousSelection checks that a previous report has unique, non-empty host names
// and records only hosts and host-fixture pairs present in selected.
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

// SelectedFixtureForPair returns the fixture selected for host and fixture ID, and whether such a pair exists.
func SelectedFixtureForPair(pairs []SelectedFixturePair, host, fixture string) (contract.Fixture, bool) {
	for _, pair := range pairs {
		if pair.Host == host && pair.Fixture.ID == fixture {
			return pair.Fixture, true
		}
	}
	return contract.Fixture{}, false
}

// CompletedForFixture counts the completed episodes for fixture.
func CompletedForFixture(episodes []contract.EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture && episode.Status == "completed" {
			count++
		}
	}
	return count
}

// AttemptsForFixture counts every episode recorded for fixture, whatever its status.
func AttemptsForFixture(episodes []contract.EpisodeReport, fixture string) int {
	count := 0
	for _, episode := range episodes {
		if episode.FixtureID == fixture {
			count++
		}
	}
	return count
}

// CountCompleted counts the episodes whose status is completed.
func CountCompleted(episodes []contract.EpisodeReport) int {
	count := 0
	for _, episode := range episodes {
		if episode.Status == "completed" {
			count++
		}
	}
	return count
}
