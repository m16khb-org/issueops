package toolconformance

import (
	"sort"

	contract "issueops/internal/contract/toolconformance"
)

type SelectedPair struct {
	Host      string
	FixtureID string
}

func DecideGate(report contract.BenchmarkReport, selected []SelectedPair, target int) contract.GateReport {
	for _, pair := range selected {
		if completedForPair(report, pair.Host, pair.FixtureID) < target {
			return contract.GateReport{Decision: contract.GateInconclusive}
		}
	}
	type signatureCount struct {
		Host      string
		FixtureID string
		Signature string
		Count     int
	}
	counts := map[string]*signatureCount{}
	for _, host := range report.Hosts {
		for _, episode := range host.Cases {
			if episode.Status != contract.EpisodeCompleted || !SchemaDriftClassification(episode.Classification) {
				continue
			}
			key := host.Host + "\x00" + episode.FixtureID + "\x00" + episode.DiagnosticSignature
			if counts[key] == nil {
				counts[key] = &signatureCount{Host: host.Host, FixtureID: episode.FixtureID, Signature: episode.DiagnosticSignature}
			}
			counts[key].Count++
		}
	}
	ordered := make([]*signatureCount, 0, len(counts))
	for _, count := range counts {
		ordered = append(ordered, count)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Host != ordered[j].Host {
			return ordered[i].Host < ordered[j].Host
		}
		if ordered[i].FixtureID != ordered[j].FixtureID {
			return ordered[i].FixtureID < ordered[j].FixtureID
		}
		return ordered[i].Signature < ordered[j].Signature
	})
	for _, count := range ordered {
		if count.Count >= 2 {
			return contract.GateReport{Decision: contract.GateAuthorizeHardening, ConfirmedSignature: count.Signature, ConfirmedCount: count.Count}
		}
	}
	if len(ordered) == 0 {
		return contract.GateReport{Decision: contract.GateDeferHardening}
	}
	if target >= 20 {
		return contract.GateReport{Decision: contract.GateUnreproducedObservation}
	}
	return contract.GateReport{Decision: contract.GateNeedsReproduction, NextReproductionTarget: ordered[0].Host + ":" + ordered[0].FixtureID}
}

func SchemaDriftClassification(classification contract.Classification) bool {
	switch classification {
	case contract.UnknownKey, contract.CoercibleTypeDrift, contract.NoncoercibleTypeDrift,
		contract.InvalidJSON, contract.MissingRequired, contract.EnumMismatch:
		return true
	default:
		return false
	}
}

func completedForPair(report contract.BenchmarkReport, host, fixture string) int {
	for _, hostReport := range report.Hosts {
		if hostReport.Host == host {
			count := 0
			for _, episode := range hostReport.Cases {
				if episode.FixtureID == fixture && episode.Status == contract.EpisodeCompleted {
					count++
				}
			}
			return count
		}
	}
	return 0
}
