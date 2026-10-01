package issueops

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// ChildCreateOperation seals one remote create without retaining the body.
type ChildCreateOperation struct {
	IssueOpsIssueCreateIntent
	Origin        string       `json:"origin"`
	RequestSHA256 string       `json:"request_sha256"`
	ParentURL     string       `json:"parent_url"`
	Generation    uint64       `json:"generation,omitempty"`
	Holder        *NativeActor `json:"holder,omitempty"`
	CWD           string       `json:"cwd,omitempty"`
}

func ValidateChildCreateOperations(operations []ChildCreateOperation) error {
	ids, implicit := map[string]bool{}, map[string]bool{}
	for _, op := range operations {
		shape := op.IssueOpsIssueCreateIntent
		shape.Marker = "<!-- issueops:issue-create:" + op.OperationID + " -->"
		if err := ValidateIssueCreateIntent(shape); err != nil {
			return err
		}
		if op.Marker != "<!-- issueops:child-create:"+op.OperationID+" -->" || (op.Origin != "implicit" && op.Origin != "explicit") || strings.TrimSpace(op.ParentURL) == "" || len(op.ParentURL) > MaxIssueCreateURLBytes {
			return fmt.Errorf("invalid child create operation identity")
		}
		if len(op.RequestSHA256) != 64 || strings.ToLower(op.RequestSHA256) != op.RequestSHA256 {
			return fmt.Errorf("invalid child request digest")
		}
		if _, err := hex.DecodeString(op.RequestSHA256); err != nil {
			return fmt.Errorf("invalid child request digest")
		}
		if ids[op.OperationID] || op.Origin == "implicit" && implicit[op.RequestSHA256] {
			return fmt.Errorf("duplicate child create operation identity")
		}
		ids[op.OperationID] = true
		if op.Origin == "implicit" {
			implicit[op.RequestSHA256] = true
		}
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
	}
	return nil
}

type ChildReconcileResult struct {
	OK                bool   `json:"ok"`
	OperationID       string `json:"operation_id"`
	ChildURL          string `json:"child_url,omitempty"`
	CandidateCount    int    `json:"candidate_count"`
	WouldAdopt        bool   `json:"would_adopt"`
	HierarchyVerified bool   `json:"hierarchy_verified"`
	RecoveryCommand   string `json:"recovery_command,omitempty"`
}
