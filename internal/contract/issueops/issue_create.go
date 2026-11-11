package issueops

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strings"

	publicationcontract "issueops/internal/contract/issueopspublication"
)

// domain/issueops는 contract/issueops만 import할 수 있어서 publication 계약의
// issue-create intent 상태를 여기서 이 이름으로 둔다.
const (
	IssueCreateIntentPending            = publicationcontract.IssueCreateIntentPending
	IssueCreateIntentNotInvoked         = publicationcontract.IssueCreateIntentNotInvoked
	IssueCreateIntentInvokedUnknown     = publicationcontract.IssueCreateIntentInvokedUnknown
	IssueCreateIntentURLObserved        = publicationcontract.IssueCreateIntentURLObserved
	IssueCreateIntentVerificationFailed = publicationcontract.IssueCreateIntentVerificationFailed
	IssueCreateIntentReceiptFailed      = publicationcontract.IssueCreateIntentReceiptFailed
	IssueCreateIntentCompleted          = publicationcontract.IssueCreateIntentCompleted

	maxIssueCreateTitleBytes     = 512
	maxIssueCreateAuthorityBytes = 512
	maxIssueCreateURLBytes       = 2048
	maxIssueCreateFailureBytes   = 2048
	maxIssueCreateValues         = 64
	maxIssueCreateValueBytes     = 256
)

// Provider subcommands an IssueCreateIntent.FailureCommand may name, and the
// HTTP status range FailureHTTPStatus may hold besides 0. The allowed commands
// only grow: removing a value makes existing records that hold it unreadable.
const (
	FailureCommandGHIssueCreate   = "gh issue create"
	FailureCommandGlabIssueCreate = "glab issue create"
	FailureCommandGlabGraphQL     = "glab api graphql"

	MinFailureHTTPStatus = 400
	MaxFailureHTTPStatus = 599
)

// issueCreateFailureCommands lists every allowed FailureCommand.
var issueCreateFailureCommands = []string{FailureCommandGHIssueCreate, FailureCommandGlabIssueCreate, FailureCommandGlabGraphQL}

// IssueCreateIntent is the sealed record of one remote issue create. The marker
// and body digest let a later run find the created issue and adopt it instead
// of creating a duplicate; Status tracks how far the create got. When a create
// fails after the provider ran, Failure is the redacted message, FailureCommand
// the provider subcommand that failed and FailureHTTPStatus the HTTP status the
// diagnostic reported; the last two are empty or 0 when unknown.
type IssueCreateIntent struct {
	OperationID       string   `json:"operation_id"`
	Marker            string   `json:"marker"`
	Provider          string   `json:"provider"`
	ProjectAuthority  string   `json:"project_authority"`
	Title             string   `json:"title"`
	BodySHA256        string   `json:"body_sha256"`
	Labels            []string `json:"labels,omitempty"`
	Assignees         []string `json:"assignees,omitempty"`
	Status            string   `json:"status"`
	Attempt           int      `json:"attempt"`
	CanonicalURL      string   `json:"canonical_url,omitempty"`
	Failure           string   `json:"failure,omitempty"`
	FailureCommand    string   `json:"failure_command,omitempty"`
	FailureHTTPStatus int      `json:"failure_http_status,omitempty"`
	StartedAt         string   `json:"started_at"`
	UpdatedAt         string   `json:"updated_at"`
}

// IssueCreateIntentRequest carries the fields a caller supplies to seal an
// IssueCreateIntent before the provider is invoked.
type IssueCreateIntentRequest struct {
	OperationID      string
	Provider         string
	ProjectAuthority string
	Title            string
	BodySHA256       string
	Labels           []string
	Assignees        []string
	StartedAt        string
}

// IssueCreateOutcome is the observed result of the provider call that updates an
// IssueCreateIntent: the new Status, the canonical URL if one was seen, and a
// Failure message otherwise, with the failed provider subcommand
// (FailureCommand) and HTTP status (FailureHTTPStatus) when they are known.
type IssueCreateOutcome struct {
	Status            string
	CanonicalURL      string
	Failure           string
	FailureCommand    string
	FailureHTTPStatus int
	ObservedAt        string
}

// IssueCreateReconcileResult reports what reconciling a pending issue create
// found: the matching candidates, whether the issue would be (or was) adopted,
// and the command that completes the recovery.
type IssueCreateReconcileResult struct {
	OK                bool               `json:"ok"`
	CandidateCount    int                `json:"candidate_count"`
	CandidateURL      string             `json:"candidate_url"`
	WouldAdopt        bool               `json:"would_adopt"`
	IssueURL          string             `json:"issue_url"`
	IssueCreateIntent *IssueCreateIntent `json:"issue_create_intent"`
	// NextCommand is the --confirm adoption a preview that found the one
	// matching issue hands back.
	NextCommand string `json:"next_command,omitempty"`
	// WouldRelease marks a preview whose next_command releases a pending
	// create whose caller died (settled: nothing was or will be created).
	WouldRelease bool `json:"would_release,omitempty"`
}

// ValidateIssueCreateIntent reports whether intent is well-formed: required
// fields, digests and provider, size limits on text and label/assignee lists,
// and a marker, attempt and status that agree with each other.
func ValidateIssueCreateIntent(intent IssueCreateIntent) error {
	if err := validateIssueCreateRequiredFields(intent); err != nil {
		return err
	}
	if err := validateIssueCreateDigestsAndProvider(intent); err != nil {
		return err
	}
	if err := validateIssueCreateFieldSizes(intent); err != nil {
		return err
	}
	if err := validateIssueCreateValues("labels", intent.Labels); err != nil {
		return err
	}
	if err := validateIssueCreateValues("assignees", intent.Assignees); err != nil {
		return err
	}
	if intent.FailureCommand != "" && !slices.Contains(issueCreateFailureCommands, intent.FailureCommand) {
		return fmt.Errorf("issue create intent failure_command is not a known provider command")
	}
	if intent.FailureHTTPStatus != 0 && (intent.FailureHTTPStatus < MinFailureHTTPStatus || intent.FailureHTTPStatus > MaxFailureHTTPStatus) {
		return fmt.Errorf("issue create intent failure_http_status must be 0 or within 400-599")
	}
	return validateIssueCreateMarkerAttemptStatus(intent)
}

func validateIssueCreateRequiredFields(intent IssueCreateIntent) error {
	for _, field := range []struct{ name, value string }{
		{"operation_id", intent.OperationID},
		{"marker", intent.Marker},
		{"provider", intent.Provider},
		{"project_authority", intent.ProjectAuthority},
		{"title", intent.Title},
		{"body_sha256", intent.BodySHA256},
		{"started_at", intent.StartedAt},
		{"updated_at", intent.UpdatedAt},
	} {
		if strings.TrimSpace(field.value) == "" {
			return fmt.Errorf("issue create intent %s is required", field.name)
		}
	}
	return nil
}

func validateIssueCreateDigestsAndProvider(intent IssueCreateIntent) error {
	if len(intent.BodySHA256) != 64 {
		return fmt.Errorf("issue create intent body_sha256 must be 64 hex characters")
	}
	if len(intent.OperationID) != 32 ||
		intent.OperationID != strings.ToLower(intent.OperationID) {
		return fmt.Errorf("issue create intent operation_id must be 32 lowercase hexadecimal characters (for example `openssl rand -hex 16`)")
	}
	if _, err := hex.DecodeString(intent.OperationID); err != nil {
		return fmt.Errorf("issue create intent operation_id must be 32 lowercase hexadecimal characters (for example `openssl rand -hex 16`)")
	}
	if _, err := hex.DecodeString(intent.BodySHA256); err != nil {
		return fmt.Errorf("issue create intent body_sha256 must be lowercase hexadecimal")
	}
	if intent.BodySHA256 != strings.ToLower(intent.BodySHA256) {
		return fmt.Errorf("issue create intent body_sha256 must be lowercase hexadecimal")
	}
	if intent.Provider != "github" && intent.Provider != "gitlab" {
		return fmt.Errorf("issue create intent provider is invalid")
	}
	return nil
}

func validateIssueCreateFieldSizes(intent IssueCreateIntent) error {
	if len(intent.ProjectAuthority) > maxIssueCreateAuthorityBytes {
		return fmt.Errorf("issue create intent project_authority exceeds %d bytes", maxIssueCreateAuthorityBytes)
	}
	if len(intent.Title) > maxIssueCreateTitleBytes {
		return fmt.Errorf("issue create intent title exceeds %d bytes", maxIssueCreateTitleBytes)
	}
	if len(intent.CanonicalURL) > maxIssueCreateURLBytes {
		return fmt.Errorf("issue create intent canonical_url exceeds %d bytes", maxIssueCreateURLBytes)
	}
	if len(intent.Failure) > maxIssueCreateFailureBytes {
		return fmt.Errorf("issue create intent failure exceeds %d bytes", maxIssueCreateFailureBytes)
	}
	if len(intent.StartedAt) > 64 || len(intent.UpdatedAt) > 64 {
		return fmt.Errorf("issue create intent timestamps exceed 64 bytes")
	}
	return nil
}

func validateIssueCreateMarkerAttemptStatus(intent IssueCreateIntent) error {
	if intent.Marker != "<!-- issueops:issue-create:"+intent.OperationID+" -->" {
		return fmt.Errorf("issue create intent marker does not match operation_id")
	}
	if intent.Attempt < 1 {
		return fmt.Errorf("issue create intent attempt must be positive")
	}
	switch intent.Status {
	case IssueCreateIntentPending,
		IssueCreateIntentNotInvoked,
		IssueCreateIntentInvokedUnknown,
		IssueCreateIntentURLObserved,
		IssueCreateIntentVerificationFailed,
		IssueCreateIntentReceiptFailed,
		IssueCreateIntentCompleted:
	default:
		return fmt.Errorf("unsupported issue create intent status %q", intent.Status)
	}
	return nil
}

func validateIssueCreateValues(field string, values []string) error {
	if len(values) > maxIssueCreateValues {
		return fmt.Errorf("issue create intent %s exceeds %d values", field, maxIssueCreateValues)
	}
	for _, value := range values {
		if len(value) > maxIssueCreateValueBytes {
			return fmt.Errorf("issue create intent %s value exceeds %d bytes", field, maxIssueCreateValueBytes)
		}
	}
	return nil
}
