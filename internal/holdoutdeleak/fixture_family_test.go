package holdoutdeleak

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"testing"
	"unicode/utf8"
)

// fixtureFingerprint identifies forbidden meeting data without committing it:
// the length and 64-bit rolling hash of the plaintext narrow the candidates and
// the SHA-256 digest confirms an exact match.
type fixtureFingerprint struct {
	length  int
	rolling uint64
	digest  string
}

// forbiddenFixtureFingerprints are fingerprints of identified meeting data that
// must never appear in any tracked file. The skills/meeting-notes fixture family
// stays synthetic.
var forbiddenFixtureFingerprints = []fixtureFingerprint{
	{6, 264263212423224, "0e3df5670eda1a187577a904e72466d118a48df6e1b81925dc7ea2d1a09a9e23"},
	{6, 266323265947531, "061cdcb5a59894767355f44c46e53da39ff9ac1778a00126d375e73883dd0ff7"},
	{6, 267491847855948, "cd07f8a79313c9b09aede56be3de3511dfab486f307fa57b0cc0df7d2f9f2016"},
	{10, 12948026869183762353, "90c1dbd608e3775069ed0d85efa3ee5de8d81b1967898c92dc4e078220687120"},
	{3, 4577505, "25f0f2b3c3a7ee2320155ec701726bac70102b7913ea2ab94db8dc28bed6a4eb"},
	{5, 310860962933, "327ff10ffaff80e02b02d84cb5d8e2224d26c5955bb5601f5c858c97eedef35b"},
	{12, 6330130600746392734, "57e699bb5ee6477c577b89f0b1e80b6ca35aa36aabc4bccd82fa2c3120ec921d"},
	{10, 12750189356887995490, "0ac875879aec9e4c761b531fe2f7b43c1dbb5bccb959bf107760f91051a6ee58"},
	{15, 10931722883136722, "b465c860684913c52bdb50b0f8391f16d96469c56fadaab652286a6595c36b36"},
	{17, 17524967233299049020, "4fb98d750c37ac63990945f14ed34a1a6b9365002efb2d5065f3f83201054fae"},
	{9, 3571922820466116722, "d04a926d0b5f5620279af27a48c70ded03df9a5216ba9bdc433acb66b71f30ec"},
	{9, 2682281289702676113, "327f595a0c22e75310f4d4f5d6cf6fa68a8180a2a86ef20f43558d6591a1fa8a"},
	{11, 18082672136676787444, "755efbe5fafe6d2d6abcd77db6691d50439dff688a983b31aa1e51f13f788720"},
	{19, 7713303281797564269, "8a73bd1e4fe0b2287b1167958c09be3a3919041bfaf9e3aa0d928b5eea20ad8d"},
	{9, 11282430282381183162, "aeac23962a5b369384bd399221cd1f01811541418b04a60bce353c1547dcb7ce"},
	{11, 117375040596452939, "ce61aa9aa1b75b926a8665183d31c8688a2b2b5a721c4420974082f8104c11c9"},
	{12, 10975145978023288744, "c053ef24eb19cfeab807405bcd0024f8c0e841fb732b408baa87164a2be7caf6"},
	{11, 15383415639973988732, "c483723d14e8691990f8d6676205166e885e803cf582e7940f4e6156437c8c1f"},
	{7, 31751348347462992, "68c212025519e0efb1009cc068490ee34edce510089ae51fbace4902c504f31a"},
	{5, 459729468423, "322888f88a59b930b950650ae9ded400a09859b52ee325d67c4427e11687a871"},
}

const fixtureRollingBase = 257

// fixtureRolling hashes data with the same polynomial the sliding scan uses.
func fixtureRolling(data []byte) uint64 {
	var rolling uint64
	for _, value := range data {
		rolling = rolling*fixtureRollingBase + uint64(value) + 1
	}
	return rolling
}

type fixtureIndexKey struct {
	length  int
	rolling uint64
}

// fixtureFingerprintIndex maps (length, rolling hash) to the confirming digest
// and lists the distinct lengths so each file is scanned once per length.
type fixtureFingerprintIndex struct {
	digests map[fixtureIndexKey]string
	lengths []int
}

func newFixtureFingerprintIndex(fingerprints []fixtureFingerprint) fixtureFingerprintIndex {
	index := fixtureFingerprintIndex{digests: make(map[fixtureIndexKey]string, len(fingerprints))}
	seen := map[int]bool{}
	for _, fingerprint := range fingerprints {
		index.digests[fixtureIndexKey{fingerprint.length, fingerprint.rolling}] = fingerprint.digest
		if !seen[fingerprint.length] {
			seen[fingerprint.length] = true
			index.lengths = append(index.lengths, fingerprint.length)
		}
	}
	return index
}

// containsFingerprint reports whether any window of content matches an indexed
// fingerprint by both rolling hash and SHA-256 digest.
func (index fixtureFingerprintIndex) containsFingerprint(content []byte) bool {
	for _, length := range index.lengths {
		if len(content) < length {
			continue
		}
		// power is base^(length-1), the weight of the byte leaving the window.
		power := uint64(1)
		for i := 1; i < length; i++ {
			power *= fixtureRollingBase
		}
		rolling := fixtureRolling(content[:length])
		for start := 0; ; start++ {
			if digest, ok := index.digests[fixtureIndexKey{length, rolling}]; ok {
				sum := sha256.Sum256(content[start : start+length])
				if hex.EncodeToString(sum[:]) == digest {
					return true
				}
			}
			if start+length == len(content) {
				break
			}
			rolling = (rolling-(uint64(content[start])+1)*power)*fixtureRollingBase + uint64(content[start+length]) + 1
		}
	}
	return false
}

var slackDocsIdentity = regexp.MustCompile(`https://([a-z0-9-]+)\.slack\.com/docs/(T[A-Z0-9]+)`)

// nonSyntheticSlackIdentities returns every Slack docs URL workspace/team pair
// other than the synthetic example/T00000000 identity.
func nonSyntheticSlackIdentities(content []byte) []string {
	var found []string
	for _, match := range slackDocsIdentity.FindAllSubmatch(content, -1) {
		if string(match[1]) != "example" || string(match[2]) != "T00000000" {
			found = append(found, string(match[1])+"/"+string(match[2]))
		}
	}
	return found
}

func TestSyntheticFixtureFamilyHasNoIdentifiedMeetingData(t *testing.T) {
	root := repoRoot(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command(git, "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files unavailable (not a work tree?): %v", err)
	}
	index := newFixtureFingerprintIndex(forbiddenFixtureFingerprints)
	for relative := range bytes.SplitSeq(out, []byte{0}) {
		if len(relative) == 0 {
			continue
		}
		path := filepath.Join(root, string(relative))
		content, err := os.ReadFile(path)
		if err != nil || !utf8.Valid(content) {
			continue
		}
		if index.containsFingerprint(content) {
			t.Errorf("%s contains identified fixture data", path)
		}
		if identities := nonSyntheticSlackIdentities(content); len(identities) > 0 {
			t.Errorf("%s contains a non-synthetic Slack workspace identity: %v", path, identities)
		}
	}
}

// The scan must actually detect what it guards against; a scan that silently
// matches nothing would pass the repository check above.
func TestFixtureScanDetectsIndexedFingerprint(t *testing.T) {
	// Spans lengths shorter than, equal to, and longer than the hash weights wrap
	// at (257^8 > 2^64), so the 64-bit wraparound path is covered.
	for _, plain := range []string{"abc", "synthetic-meeting-fixture", "n0t-a-real-attendee-name-0123456789"} {
		sum := sha256.Sum256([]byte(plain))
		index := newFixtureFingerprintIndex([]fixtureFingerprint{{
			length:  len(plain),
			rolling: fixtureRolling([]byte(plain)),
			digest:  hex.EncodeToString(sum[:]),
		}})
		for name, content := range map[string]string{
			"whole file": plain,
			"prefix":     plain + " trailing text",
			"suffix":     "leading text " + plain,
			"middle":     "leading text " + plain + " trailing text",
		} {
			if !index.containsFingerprint([]byte(content)) {
				t.Errorf("%q: %s not detected", plain, name)
			}
		}
		if index.containsFingerprint([]byte("unrelated content of ordinary length")) {
			t.Errorf("%q: unrelated content reported as a match", plain)
		}
		if len(plain) > 1 && index.containsFingerprint([]byte(plain[:len(plain)-1])) {
			t.Errorf("%q: content shorter than the fingerprint reported as a match", plain)
		}
	}
}

func TestFixtureScanRequiresDigestConfirmation(t *testing.T) {
	plain := []byte("synthetic-meeting-fixture")
	index := newFixtureFingerprintIndex([]fixtureFingerprint{{
		length:  len(plain),
		rolling: fixtureRolling(plain),
		digest:  hex.EncodeToString(make([]byte, sha256.Size)),
	}})
	if index.containsFingerprint(plain) {
		t.Error("rolling-hash match without SHA-256 confirmation must not be reported")
	}
}

func TestSlackIdentityScanAllowsOnlySyntheticWorkspace(t *testing.T) {
	synthetic := []byte("see https://example.slack.com/docs/T00000000/F123")
	if got := nonSyntheticSlackIdentities(synthetic); len(got) != 0 {
		t.Errorf("synthetic identity flagged: %v", got)
	}
	for _, content := range []string{
		"https://acme-corp.slack.com/docs/T01ABCDEF/F123",
		"https://example.slack.com/docs/T01ABCDEF/F123",
		"https://acme.slack.com/docs/T00000000/F123",
	} {
		if got := nonSyntheticSlackIdentities([]byte(content)); len(got) != 1 {
			t.Errorf("%q: want one flagged identity, got %v", content, got)
		}
	}
}
