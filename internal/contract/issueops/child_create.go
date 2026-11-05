package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ChildCreateOperation seals one remote create without retaining the body.
type ChildCreateOperation struct {
	IssueCreateIntent
	Origin        string       `json:"origin"`
	RequestSHA256 string       `json:"request_sha256"`
	ParentURL     string       `json:"parent_url"`
	Generation    uint64       `json:"generation,omitempty"`
	Holder        *NativeActor `json:"holder,omitempty"`
	CWD           string       `json:"cwd,omitempty"`
}

// ValidateChildCreateOperations checks every sealed child-create operation: the
// embedded issue-create shape, marker and origin identity, request digest, the
// canonical URL required once the create completed, and that a leased operation
// carries its holder and CWD (an unleased one carries neither). It rejects a
// duplicate operation ID and a duplicate implicit request digest.
func ValidateChildCreateOperations(operations []ChildCreateOperation) error {
	ids, implicit := map[string]bool{}, map[string]bool{}
	for _, op := range operations {
		shape := op.IssueCreateIntent
		shape.Marker = "<!-- issueops:issue-create:" + op.OperationID + " -->"
		if err := ValidateIssueCreateIntent(shape); err != nil {
			return err
		}
		if err := validateChildCreateIdentity(op); err != nil {
			return err
		}
		if err := validateChildRequestDigest(op.RequestSHA256); err != nil {
			return err
		}
		if ids[op.OperationID] || op.Origin == "implicit" && implicit[op.RequestSHA256] {
			return fmt.Errorf("duplicate child create operation identity")
		}
		ids[op.OperationID] = true
		if op.Origin == "implicit" {
			implicit[op.RequestSHA256] = true
		}
		if err := validateChildCreateCanonicalAndAuthority(op); err != nil {
			return err
		}
	}
	return nil
}

func validateChildCreateIdentity(op ChildCreateOperation) error {
	if op.Marker != "<!-- issueops:child-create:"+op.OperationID+" -->" || (op.Origin != "implicit" && op.Origin != "explicit") || strings.TrimSpace(op.ParentURL) == "" || len(op.ParentURL) > maxIssueCreateURLBytes {
		return fmt.Errorf("invalid child create operation identity")
	}
	return nil
}

func validateChildRequestDigest(digest string) error {
	if len(digest) != 64 || strings.ToLower(digest) != digest {
		return fmt.Errorf("invalid child request digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return fmt.Errorf("invalid child request digest")
	}
	return nil
}

func validateChildCreateCanonicalAndAuthority(op ChildCreateOperation) error {
	if (op.Status == IssueCreateIntentCompleted || op.Status == IssueCreateIntentURLObserved) && op.CanonicalURL == "" {
		return fmt.Errorf("child create operation requires canonical_url")
	}
	if op.Generation == 0 {
		if op.Holder != nil || op.CWD != "" {
			return fmt.Errorf("invalid unleased child authority")
		}
	} else if op.Holder == nil || op.CWD == "" {
		return fmt.Errorf("missing child authority")
	}
	return nil
}

// ChildReconcileResult reports what reconciling a pending child create found:
// how many remote candidates matched, whether the child would be (or was)
// adopted, and the command that finishes recovery.
type ChildReconcileResult struct {
	OK                bool   `json:"ok"`
	OperationID       string `json:"operation_id"`
	ChildURL          string `json:"child_url,omitempty"`
	CandidateCount    int    `json:"candidate_count"`
	WouldAdopt        bool   `json:"would_adopt"`
	HierarchyVerified bool   `json:"hierarchy_verified"`
	RecoveryCommand   string `json:"recovery_command,omitempty"`
	// Released reports that a pending create whose caller died was settled
	// as never created and released (not_invoked): re-run the same
	// create-child with this operation ID.
	Released bool `json:"released,omitempty"`
	// WouldRelease marks a preview whose recovery_command releases it.
	WouldRelease bool `json:"would_release,omitempty"`
}
