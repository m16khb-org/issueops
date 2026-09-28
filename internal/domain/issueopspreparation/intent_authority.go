package issueopspreparation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func ValidateIntentRecord(record preparationcontract.Record, intent preparationcontract.Intent) error {
	if err := (preparationcontract.IntentCodec{}).Validate(intent, intent.OperationID); err != nil {
		return err
	}
	return ValidateIntentRecordAuthority(record, intent)
}

// CanonicalizeIntent checks the existing bytes without changing the persisted payload.
func CanonicalizeIntent(record preparationcontract.Record, raw []byte) (preparationcontract.Intent, []byte, error) {
	intent, err := (preparationcontract.IntentCodec{}).DecodeSelfIdentified(raw)
	if err != nil {
		return preparationcontract.Intent{}, nil, err
	}
	if err := ValidateIntentRecordAuthority(record, intent); err != nil {
		return preparationcontract.Intent{}, nil, err
	}
	return intent, append([]byte(nil), raw...), nil
}

func normalizedPurpose(intent preparationcontract.Intent) string {
	if strings.TrimSpace(intent.Purpose) == "" {
		return preparationcontract.PurposePrepare
	}
	return strings.TrimSpace(intent.Purpose)
}

func contractError(code, detail string) error {
	return &preparationcontract.IntentError{Code: code, Detail: detail}
}
func validProvider(value string) bool { return value == "github" || value == "gitlab" }

func ValidateIntentRecordAuthority(record preparationcontract.Record, intent preparationcontract.Intent) error {
	if record.ID != intent.LifecycleID || record.Execution == nil || record.Execution.Pending == nil ||
		record.Execution.Pending.OperationID != intent.OperationID || record.Execution.Pending.Marker != intent.Marker ||
		record.Execution.Pending.Kind != PendingKind(intent.Stage) || record.Execution.Lease.Generation != intent.Generation {
		return fmt.Errorf("Orca intent authority changed before CAS")
	}
	switch normalizedPurpose(intent) {
	case preparationcontract.PurposePrepare:
		if record.Execution.Lease.Status != "released" || record.Execution.Orca != nil {
			return fmt.Errorf("Orca prepare intent authority changed before CAS")
		}
	case preparationcontract.PurposeResume:
		if intent.ResumeLease == nil || intent.PriorBinding == nil ||
			!leasesEqual(record.Execution.Lease, *intent.ResumeLease) || !bindingsEqual(record.Execution.Orca, intent.PriorBinding) {
			return fmt.Errorf("Orca resume intent authority changed before CAS")
		}
	default:
		return fmt.Errorf("unsupported Orca intent purpose")
	}
	return nil
}

func PrepareIssueIdentity(recordIssueURL string, prepared *preparationcontract.IssueLinkEvidence) (preparationcontract.IssueIdentity, error) {
	if prepared == nil {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires verified branch issue identity")
	}
	provider := strings.ToLower(strings.TrimSpace(prepared.Provider))
	if !prepared.LinkVerified && provider != "github" {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires verified branch issue identity")
	}
	if !validProvider(provider) || strings.TrimSpace(prepared.IssueURL) == "" || strings.TrimSpace(recordIssueURL) != strings.TrimSpace(prepared.IssueURL) {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent issue URL does not match the verified branch identity")
	}
	parsed, err := url.Parse(prepared.IssueURL)
	if err != nil || parsed.Hostname() == "" || !providerIssueURLMatches(provider, parsed) {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent provider does not match the verified issue URL")
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 2 || (parts[len(parts)-2] != "issues" && parts[len(parts)-2] != "work_items") {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires a positive issue number")
	}
	issue, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || issue <= 0 {
		return preparationcontract.IssueIdentity{}, contractError("intent_identity_mismatch", "Orca intent requires a positive issue number")
	}
	return preparationcontract.IssueIdentity{Provider: provider, Issue: issue}, nil
}

// SealIntent binds the verified issue identity to an external intent and its marker.
func SealIntent(intent preparationcontract.Intent, issue preparationcontract.IssueIdentity) (preparationcontract.Intent, error) {
	if intent.LifecycleID == "" || !validProvider(issue.Provider) || issue.Issue <= 0 {
		return preparationcontract.Intent{}, contractError("intent_identity_mismatch", "Orca intent issue identity is invalid")
	}
	intent.Probe.Provider = strings.ToLower(strings.TrimSpace(issue.Provider))
	intent.Probe.Issue = issue.Issue
	codec := preparationcontract.IntentCodec{}
	marker, err := codec.RenderMarker(preparationcontract.MarkerIdentity{
		Purpose: normalizedPurpose(intent), LifecycleID: intent.LifecycleID,
		Generation: intent.Generation, OperationID: intent.OperationID,
		Provider: intent.Probe.Provider, Issue: intent.Probe.Issue,
	})
	if err != nil {
		return preparationcontract.Intent{}, err
	}
	intent.Marker = marker
	intent.Probe.Marker = marker
	if err := codec.Validate(intent, intent.OperationID); err != nil {
		return preparationcontract.Intent{}, err
	}
	return intent, nil
}

func PendingKind(stage preparationcontract.IntentStage) string {
	switch stage {
	case preparationcontract.IntentStageWorktree:
		return "worktree_create"
	case preparationcontract.IntentStageTerminal, preparationcontract.IntentStageRun, preparationcontract.IntentStageRunBind, preparationcontract.IntentStageTask:
		return "owner_launch"
	case preparationcontract.IntentStageDispatch:
		return "dispatch"
	default:
		return ""
	}
}

func leasesEqual(left, right preparationcontract.Lease) bool {
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	return bytes.Equal(leftRaw, rightRaw)
}

func bindingsEqual(left *preparationcontract.OrcaBinding, right *preparationcontract.ResumeBinding) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.RuntimeID == right.RuntimeID && left.RepoID == right.RepoID && left.WorktreeID == right.WorktreeID &&
		left.WorktreeInstanceID == right.WorktreeInstanceID && left.LeaseGeneration == right.LeaseGeneration &&
		left.OwnerHost == right.OwnerHost && left.OwnerModel == right.OwnerModel && left.OwnerEffort == right.OwnerEffort &&
		left.RunID == right.RunID && left.TaskID == right.TaskID && left.DispatchID == right.DispatchID && left.TerminalPTYID == right.TerminalPTYID
}

func providerIssueURLMatches(provider string, parsed *url.URL) bool {
	host, path := strings.ToLower(parsed.Hostname()), strings.ToLower(parsed.Path)
	switch provider {
	case "github":
		return host == "github.com" && strings.Contains(path, "/issues/")
	case "gitlab":
		return strings.Contains(host, "gitlab") || strings.Contains(path, "/-/issues/") || strings.Contains(path, "/-/work_items/")
	default:
		return false
	}
}
