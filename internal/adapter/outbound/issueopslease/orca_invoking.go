package issueopslease

import (
	"context"
	"errors"
	"fmt"

	preparationapp "issueops/internal/application/issueopspreparation"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
	"issueops/internal/port"
)

func markOrcaIntentInvoking(ctx context.Context, store port.TransactionalRecordStore, record leasecontract.Record, operationID string, recordRaw, intentRaw []byte) (preparationcontract.Intent, []byte, error) {
	if store == nil {
		return preparationcontract.Intent{}, nil, fmt.Errorf("transactional record store is required")
	}
	cas, ok := store.(port.RecordRawCASStore)
	if !ok {
		return preparationcontract.Intent{}, nil, fmt.Errorf("resume record store does not support raw CAS")
	}
	if len(recordRaw) == 0 || len(intentRaw) == 0 {
		return preparationcontract.Intent{}, nil, fmt.Errorf("Orca intent raw CAS evidence is required")
	}
	codec := preparationcontract.IntentCodec{}
	current, err := codec.Decode(operationID, intentRaw)
	if err != nil {
		return preparationcontract.Intent{}, nil, err
	}
	if err := preparationdomain.ValidateIntentRecord(record, current); err != nil {
		return preparationcontract.Intent{}, nil, err
	}
	updated := preparationapp.MarkOrcaInvoking(current)
	data, err := codec.Encode(updated)
	if err != nil {
		return preparationcontract.Intent{}, nil, err
	}
	err = cas.CompareAndApply(ctx, []port.ExpectedRecord{
		{Bucket: recordBucket, ID: record.ID, Data: recordRaw},
		{Bucket: "external_intent_v1", ID: operationID, Data: intentRaw},
	}, []port.RecordMutation{{Bucket: "external_intent_v1", ID: operationID, Data: data}})
	if stale, ok := errors.AsType[port.RawCASFailure](err); ok {
		if stale.FailedBucket() == recordBucket {
			return preparationcontract.Intent{}, nil, fmt.Errorf("stale raw record snapshot")
		}
		return preparationcontract.Intent{}, nil, fmt.Errorf("stale raw intent snapshot")
	}
	return updated, data, err
}
