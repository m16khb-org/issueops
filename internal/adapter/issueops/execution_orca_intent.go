package issueops

import (
	"errors"
	"fmt"
	"os"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	preparationcontract "issueops/internal/contract/issueopspreparation"
)

func ReadExecutionOrcaIntent(stateRoot, operationID string) (preparationcontract.Intent, error) {
	db, err := sqlstore.Open(stateRoot)
	if err != nil {
		return preparationcontract.Intent{}, err
	}
	data, ok, err := db.Get(externalIntentBucket, operationID)
	if err != nil {
		return preparationcontract.Intent{}, err
	}
	if !ok {
		return preparationcontract.Intent{}, fmt.Errorf("Orca external intent payload is missing")
	}
	return (preparationcontract.IntentCodec{}).Decode(operationID, data)
}

func createOrAdoptClaimToken(record issueops.IssueOpsRecord) (string, error) {
	token, _, err := createClaimToken(record)
	if err == nil {
		return tokenSHA256(token), nil
	}
	if !errors.Is(err, os.ErrExist) {
		return "", err
	}
	token, err = readExecutionLeaseToken(record, claimTokenPath(record))
	if err != nil {
		return "", fmt.Errorf("recover deterministic claim token: %w", err)
	}
	return tokenSHA256(token), nil
}
