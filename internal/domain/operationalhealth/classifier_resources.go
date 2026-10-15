package operationalhealth

import (
	"fmt"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"sort"
	"strings"
)

// anyTrue reports whether any condition holds; every operand is a side-effect-free comparison.
func anyTrue(conditions ...bool) bool {
	for _, condition := range conditions {
		if condition {
			return true
		}
	}
	return false
}

func sameTrimmed(a, b string) bool {
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

func validateCycleGitWorktree(builder *findingBuilder, resources resourceIndex, cycle operationalhealthcontract.Cycle, authority operationalhealthcontract.CycleAuthority, repoScoped bool, gitPathCounts map[string]int) {
	if !repoScoped || (authority != AuthorityLive && strings.TrimSpace(cycle.WorktreePath) == "") {
		return
	}
	gitWorktree, ok := resources.gitPaths.unique(cycle.WorktreePath)
	if anyTrue(!ok, gitPathCounts[clean(cycle.WorktreePath)] != 1, !sameTrimmed(gitWorktree.Branch, cycle.Branch)) {
		builder.add(operationalhealthcontract.FindingInventoryUnknown, "git_worktree", clean(cycle.WorktreePath), "cycle identity does not match exactly one Git worktree", clean(cycle.WorktreePath))
	}
}

func validateLeaseHolderIndexes(builder *findingBuilder, cycles []operationalhealthcontract.Cycle, indexes []operationalhealthcontract.LeaseHolderIndex) {
	active := make([]operationalhealthcontract.Cycle, 0, len(cycles))
	holderOwners := make(map[string][]string)
	activeCounts := make(map[leaseHolderKey]int)
	indexCounts := make(map[leaseHolderKey]int)
	for _, index := range indexes {
		indexCounts[holderKey(index.LifecycleID, index.Generation, index.Host, index.SessionID, index.AgentID)]++
	}
	for _, cycle := range cycles {
		if strings.TrimSpace(cycle.LeaseStatus) != "active" {
			continue
		}
		active = append(active, cycle)
		activeCounts[holderKey(cycle.ID, cycle.Generation, cycle.HolderHost, cycle.HolderSessionID, cycle.HolderAgentID)]++
		addOwner(holderOwners, nativeHolderIdentity(cycle.HolderHost, cycle.HolderSessionID, cycle.HolderAgentID), cycle.ID)
	}
	for identity, owners := range holderOwners {
		if identity != "" && len(owners) > 1 {
			sort.Strings(owners)
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "lease_holder", identity, "native session is recorded as holder of multiple active cycles: "+strings.Join(owners, ","), "")
		}
	}
	keyCounts := countBy(indexes, func(index operationalhealthcontract.LeaseHolderIndex) string { return strings.TrimSpace(index.Key) })
	addDuplicateFindings(builder, "lease_holder", keyCounts)
	for _, cycle := range active {
		matches := indexCounts[holderKey(cycle.ID, cycle.Generation, cycle.HolderHost, cycle.HolderSessionID, cycle.HolderAgentID)]
		if matches != 1 {
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "lease_holder", strings.TrimSpace(cycle.ID), "active cycle must match exactly one lease-holder reverse index", "")
		}
	}
	validateLeaseHolderIndexEntries(builder, indexes, activeCounts)
}

func leaseHolderIndexComplete(index operationalhealthcontract.LeaseHolderIndex) bool {
	return strings.TrimSpace(index.Key) != "" && strings.TrimSpace(index.LifecycleID) != "" && index.Generation > 0 &&
		validNativeHost(index.Host) && strings.TrimSpace(index.SessionID) != ""
}

func validateLeaseHolderIndexEntries(builder *findingBuilder, indexes []operationalhealthcontract.LeaseHolderIndex, activeCounts map[leaseHolderKey]int) {
	for _, index := range indexes {
		matches := activeCounts[holderKey(index.LifecycleID, index.Generation, index.Host, index.SessionID, index.AgentID)]
		if !leaseHolderIndexComplete(index) || matches != 1 {
			builder.add(operationalhealthcontract.FindingInventoryUnknown, "lease_holder", firstNonEmpty(strings.TrimSpace(index.Key), strings.TrimSpace(index.LifecycleID), "index"), "lease-holder reverse index must match exactly one active cycle", "")
		}
	}
}

type leaseHolderKey struct {
	lifecycleID string
	generation  uint64
	host        string
	sessionID   string
	agentID     string
}

func holderKey(lifecycleID string, generation uint64, host, sessionID, agentID string) leaseHolderKey {
	return leaseHolderKey{strings.TrimSpace(lifecycleID), generation, strings.TrimSpace(host), strings.TrimSpace(sessionID), strings.TrimSpace(agentID)}
}

func nativeHolderIdentity(host, sessionID, agentID string) string {
	host, sessionID, agentID = strings.TrimSpace(host), strings.TrimSpace(sessionID), strings.TrimSpace(agentID)
	if host == "" || sessionID == "" {
		return ""
	}
	return strings.ToLower(host) + "\x00" + sessionID + "\x00" + agentID
}

func preserveContains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, value := range values {
		if strings.TrimSpace(value) == want && want != "" {
			return true
		}
	}
	return false
}

func preserveSet(values []string) map[string]bool {
	result, _ := normalizedSet(values)
	return result
}

func normalizedSet(values []string) (map[string]bool, []string) {
	result := make(map[string]bool, len(values))
	invalid := make([]string, 0)
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" || result[value] {
			invalid = append(invalid, value)
			continue
		}
		result[value] = true
	}
	return result, invalid
}

func ownerIndex(cycles []operationalhealthcontract.Cycle, identity func(operationalhealthcontract.Cycle) string) map[string][]string {
	owners := make(map[string][]string)
	for _, cycle := range cycles {
		addOwner(owners, identity(cycle), cycle.ID)
	}
	for id := range owners {
		sort.Strings(owners[id])
	}
	return owners
}

func addOwner(owners map[string][]string, resourceID, cycleID string) {
	resourceID = strings.TrimSpace(resourceID)
	if resourceID == "" {
		return
	}
	owners[resourceID] = append(owners[resourceID], strings.TrimSpace(cycleID))
}

func countBy[T any](values []T, identity func(T) string) map[string]int {
	counts := make(map[string]int)
	for _, value := range values {
		if id := identity(value); id != "" {
			counts[id]++
		}
	}
	return counts
}

func addDuplicateFindings(builder *findingBuilder, kind string, counts map[string]int) {
	for id, count := range counts {
		if count > 1 {
			builder.add(operationalhealthcontract.FindingInventoryUnknown, kind, id, fmt.Sprintf("%s identity occurs %d times", kind, count), "")
		}
	}
}

type resourceBucket[T any] struct {
	count int
	value T
}

type resourceLookup[T any] map[string]resourceBucket[T]

func indexBy[T any](values []T, identity func(T) string) resourceLookup[T] {
	index := make(resourceLookup[T], len(values))
	for _, value := range values {
		key := strings.TrimSpace(identity(value))
		bucket := index[key]
		bucket.count++
		if bucket.count == 1 {
			bucket.value = value
		} else {
			var zero T
			bucket.value = zero
		}
		index[key] = bucket
	}
	return index
}

func (index resourceLookup[T]) unique(want string) (T, bool) {
	bucket := index[strings.TrimSpace(want)]
	return bucket.value, bucket.count == 1
}

func (index resourceLookup[T]) counts() map[string]int {
	counts := make(map[string]int, len(index))
	for key, bucket := range index {
		// countBy ignored empty identities; uniqueBy still matched them.
		if key != "" {
			counts[key] = bucket.count
		}
	}
	return counts
}

type resourceIndex struct {
	cycles   resourceLookup[operationalhealthcontract.Cycle]
	gitPaths resourceLookup[operationalhealthcontract.GitWorktree]
}

func newResourceIndex(snapshot Snapshot) resourceIndex {
	return resourceIndex{
		cycles:   indexBy(snapshot.Cycles, func(cycle operationalhealthcontract.Cycle) string { return cycle.ID }),
		gitPaths: indexBy(snapshot.GitWorktrees, func(worktree operationalhealthcontract.GitWorktree) string { return clean(worktree.Path) }),
	}
}

func clean(path string) string {
	return strings.TrimSpace(path)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

type findingBuilder struct {
	findings []operationalhealthcontract.Finding
	seen     map[string]struct{}
}

func (builder *findingBuilder) add(code, kind, id, summary, path string) {
	finding := operationalhealthcontract.Finding{Code: code, ResourceKind: kind, ResourceID: id, Summary: summary, Path: path}
	key := strings.Join([]string{code, kind, id, path, summary}, "\x00")
	if _, exists := builder.seen[key]; exists {
		return
	}
	builder.seen[key] = struct{}{}
	builder.findings = append(builder.findings, finding)
}

func (builder *findingBuilder) sorted() []operationalhealthcontract.Finding {
	sort.Slice(builder.findings, func(i, j int) bool {
		left := builder.findings[i]
		right := builder.findings[j]
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.ResourceKind != right.ResourceKind {
			return left.ResourceKind < right.ResourceKind
		}
		if left.ResourceID != right.ResourceID {
			return left.ResourceID < right.ResourceID
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return left.Summary < right.Summary
	})
	return builder.findings
}
