package issueopslease

import (
	"context"
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

const claimContextIssueBody = "linked issue body"

type claimContextReaderFake struct {
	record      leasecontract.Record
	loadErr     error
	packetData  []byte
	packetPath  string
	packetErr   error
	packet      leasecontract.ClaimContextPacket
	decodeErr   error
	artifacts   map[string][]byte
	artifactErr error
	packetReads int
}

func (f *claimContextReaderFake) Load(string) (leasecontract.Record, error) {
	return f.record, f.loadErr
}

func (f *claimContextReaderFake) ReadPacket(leasecontract.Record) ([]byte, string, error) {
	f.packetReads++
	return f.packetData, f.packetPath, f.packetErr
}

func (f *claimContextReaderFake) DecodePacket([]byte) (leasecontract.ClaimContextPacket, error) {
	return f.packet, f.decodeErr
}

func (f *claimContextReaderFake) ReadArtifact(_ leasecontract.Record, name string) ([]byte, error) {
	if f.artifactErr != nil {
		return nil, f.artifactErr
	}
	return f.artifacts[name], nil
}

type claimContextFixture struct {
	reader      *claimContextReaderFake
	issueDigest string
	packetSum   string
	request     ClaimPreflightRequest
	snapshot    IssueSnapshot
	snapshotErr error
	reads       int
}

func newClaimContextFixture() *claimContextFixture {
	artifact := []byte("owner prompt")
	issueDigest := leasedomain.ClaimContextDigest([]byte(claimContextIssueBody))
	data := []byte(`{"sealed":"packet"}`)
	record := leasecontract.Record{ID: "io-sealed", Repo: "/repo", IssueURL: "https://example.test/issues/1", Execution: &leasecontract.Execution{
		Mode:      "orca",
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: "base"},
		Lease:     leasecontract.Lease{Generation: 3, Status: "claimable"},
	}}
	fixture := &claimContextFixture{
		issueDigest: issueDigest,
		packetSum:   leasedomain.ClaimContextDigest(data),
		snapshot:    IssueSnapshot{URL: record.IssueURL, Body: claimContextIssueBody},
		reader: &claimContextReaderFake{
			record: record, packetData: data, packetPath: "/worktree/packet.json",
			packet: leasecontract.ClaimContextPacket{
				SchemaVersion: leasecontract.SchemaVersion, LifecycleID: record.ID, Mode: "orca", SourceRoot: "/source", WorktreeRoot: "/worktree",
				Branch: "branch", BaseHead: "base", LeaseGeneration: 3,
				Issue:            leasecontract.ClaimPacketIssue{URL: record.IssueURL, Body: claimContextIssueBody, BodySHA256: issueDigest},
				ArtifactManifest: map[string]string{"prompt.md": leasedomain.ClaimContextDigest(artifact)},
			},
			artifacts: map[string][]byte{"prompt.md": artifact},
		},
	}
	fixture.request = ClaimPreflightRequest{ID: record.ID, Generation: 3, IssueBodySHA256: issueDigest, ContextPacketSHA256: fixture.packetSum}
	return fixture
}

func (f *claimContextFixture) context() *SealedClaimContext {
	read := func(_ context.Context, repo, url string) (IssueSnapshot, error) {
		f.reads++
		if repo != "/repo" || url != f.reader.record.IssueURL {
			return IssueSnapshot{}, errors.New("unexpected issue coordinates " + repo + " " + url)
		}
		return f.snapshot, f.snapshotErr
	}
	return NewSealedClaimContext(f.reader, read, reseedPathMatcher{})
}

func TestSealedClaimContextSkipsNonOrcaAndStaleGenerationWithoutReadingSealedState(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*claimContextFixture)
	}{
		{"direct mode", func(f *claimContextFixture) { f.reader.record.Execution.Mode = "direct" }},
		{"generation differs from lease", func(f *claimContextFixture) { f.request.Generation = 2 }},
		{"zero generation", func(f *claimContextFixture) { f.request.Generation = 0 }},
		{"no execution", func(f *claimContextFixture) { f.reader.record.Execution = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newClaimContextFixture()
			tt.mutate(fixture)
			validate, err := fixture.context().Preflight(t.Context(), fixture.request)
			if err != nil {
				t.Fatalf("preflight: %v", err)
			}
			if err := validate(Record{}); err != nil {
				t.Fatalf("pass-through validator rejected: %v", err)
			}
			if fixture.reader.packetReads != 0 || fixture.reads != 0 {
				t.Fatalf("sealed state read packet=%d issue=%d", fixture.reader.packetReads, fixture.reads)
			}
		})
	}
}

func TestSealedClaimContextPreflightRejectsDriftedOrUnreadableSealedState(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(*claimContextFixture)
		want       string
		wantIssues int
	}{
		{"load failure", func(f *claimContextFixture) { f.reader.loadErr = errors.New("record missing") }, "record missing", 0},
		{"short digests", func(f *claimContextFixture) { f.request.IssueBodySHA256 = "abc" }, "sealed issue and context packet digests", 0},
		{"packet unreadable", func(f *claimContextFixture) { f.reader.packetErr = errors.New("disk gone") }, "read sealed context packet: disk gone", 0},
		{"packet digest mismatch", func(f *claimContextFixture) { f.reader.packetData = []byte("tampered") }, "sealed context packet digest mismatch", 0},
		{"packet undecodable", func(f *claimContextFixture) { f.reader.decodeErr = errors.New("bad json") }, "parse sealed context packet: bad json", 0},
		{"packet worktree differs", func(f *claimContextFixture) { f.reader.packet.WorktreeRoot = "/elsewhere" }, "execution identity mismatch", 0},
		{"packet source differs", func(f *claimContextFixture) { f.reader.packet.SourceRoot = "/elsewhere" }, "execution identity mismatch", 0},
		{"packet generation differs", func(f *claimContextFixture) { f.reader.packet.LeaseGeneration = 2 }, "packet_generation=2 expected_generation=3", 0},
		{"packet issue digest differs from request", func(f *claimContextFixture) {
			f.reader.packet.Issue.BodySHA256 = strings.Repeat("0", 64)
		}, "issue body digest mismatch", 0},
		{"packet body does not hash to digest", func(f *claimContextFixture) { f.reader.packet.Issue.Body = "edited body" }, "does not hash to its sealed digest", 0},
		{"artifact unreadable", func(f *claimContextFixture) { f.reader.artifactErr = errors.New("no artifact") }, "read sealed artifact prompt.md: no artifact", 0},
		{"artifact digest mismatch", func(f *claimContextFixture) { f.reader.artifacts["prompt.md"] = []byte("edited prompt") }, "sealed artifact prompt.md digest mismatch", 0},
		{"remote reader unavailable", func(f *claimContextFixture) {}, "remote issue snapshot reader is unavailable", 0},
		{"remote read failure", func(f *claimContextFixture) { f.snapshotErr = errors.New("rate limited") }, "read remote issue before claim: rate limited", 1},
		{"remote url differs", func(f *claimContextFixture) { f.snapshot.URL = "https://example.test/issues/2" }, "snapshot url does not match", 1},
		{"remote body drifted", func(f *claimContextFixture) { f.snapshot.Body = "edited upstream" }, "issue body digest drifted", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newClaimContextFixture()
			tt.mutate(fixture)
			claim := fixture.context()
			if tt.name == "remote reader unavailable" {
				claim = NewSealedClaimContext(fixture.reader, nil, reseedPathMatcher{})
			}
			validate, err := claim.Preflight(t.Context(), fixture.request)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("preflight error=%v want substring %q", err, tt.want)
			}
			if validate != nil {
				t.Fatal("rejected preflight must not return a validator")
			}
			if fixture.reads != tt.wantIssues {
				t.Fatalf("remote issue reads=%d want %d", fixture.reads, tt.wantIssues)
			}
		})
	}
}

func TestSealedClaimContextValidatorRechecksSealedStateAfterPreflight(t *testing.T) {
	fixture := newClaimContextFixture()
	validate, err := fixture.context().Preflight(t.Context(), fixture.request)
	if err != nil {
		t.Fatalf("preflight: %v", err)
	}
	if fixture.reads != 1 {
		t.Fatalf("remote issue reads=%d want 1", fixture.reads)
	}
	current := Record{Stable: fixture.reader.record}
	if err := validate(current); err != nil {
		t.Fatalf("unchanged sealed state rejected: %v", err)
	}

	// The packet on disk changes between preflight and the locked claim.
	fixture.reader.packetData = []byte("rewritten after preflight")
	if err := validate(current); err == nil || !strings.Contains(err.Error(), "sealed context packet digest mismatch") {
		t.Fatalf("drift after preflight accepted: %v", err)
	}
	fixture.reader.packetData = []byte(`{"sealed":"packet"}`)

	// The record itself stopped being an Orca generation.
	current.Stable.Execution = nil
	if err := validate(current); err == nil || !strings.Contains(err.Error(), "no longer matches an Orca execution generation") {
		t.Fatalf("non-orca record accepted: %v", err)
	}
}
