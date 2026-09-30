package issueopspublication

import (
	"fmt"
	"strings"
)

func ValidateOperationID(operationID string) error {
	if len(operationID) == 32 {
		valid := true
		for _, char := range []byte(operationID) {
			if (char < '0' || char > '9') && (char < 'a' || char > 'f') {
				valid = false
				break
			}
		}
		if valid {
			return nil
		}
	}
	return fmt.Errorf("remote operation ID must be exactly 32 lowercase hexadecimal characters")
}

type BeginAuthorityFacts struct {
	Prepared           bool
	Pending            bool
	Artifact           bool
	CurrentGeneration  uint64
	ExpectedGeneration uint64
}

func ValidateBeginAuthority(facts BeginAuthorityFacts) error {
	if !facts.Prepared || facts.Pending || facts.Artifact {
		return fmt.Errorf("remote create authority changed before intent CAS")
	}
	if facts.ExpectedGeneration == 0 || facts.CurrentGeneration != facts.ExpectedGeneration {
		return fmt.Errorf("stale lease generation before remote intent CAS")
	}
	return nil
}

type ReceiptAuthorityFacts struct {
	Prepared            bool
	Pending             bool
	PendingOperationID  string
	ExpectedOperationID string
	Generation          uint64
	ExpectedGeneration  uint64
	LeaseStatus         string
	HolderPresent       bool
	HolderHost          string
	ExpectedHost        string
	HolderSessionID     string
	ExpectedSessionID   string
	HolderAgentID       string
	ExpectedAgentID     string
	CWDMatches          bool
}

func ValidateReceiptAuthority(facts ReceiptAuthorityFacts, enforceOriginalGeneration bool) error {
	if !facts.Prepared || !facts.Pending || facts.PendingOperationID != facts.ExpectedOperationID {
		return fmt.Errorf("external intent changed before remote receipt CAS")
	}
	if enforceOriginalGeneration && (facts.ExpectedGeneration == 0 || facts.Generation != facts.ExpectedGeneration ||
		facts.LeaseStatus != "active" || !facts.HolderPresent ||
		!strings.EqualFold(facts.HolderHost, facts.ExpectedHost) || facts.HolderSessionID != facts.ExpectedSessionID ||
		facts.HolderAgentID != facts.ExpectedAgentID || !facts.CWDMatches) {
		return fmt.Errorf("remote receipt belongs to a stale execution generation; execution reconcile is required")
	}
	return nil
}
