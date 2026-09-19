package audit

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/auditid"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/policy"
)

type HandoffDeliveryAuditRecord struct {
	OK            bool                                                `json:"ok"`
	Kind          string                                              `json:"kind"`
	SchemaVersion int                                                 `json:"schema_version"`
	AuditLogID    string                                              `json:"audit_log_id"`
	GeneratedAt   string                                              `json:"generated_at"`
	LogPath       string                                              `json:"log_path,omitempty"`
	RecordDigest  string                                              `json:"record_digest"`
	Observation   issueopscontract.IssueOpsHandoffDeliveryObservation `json:"observation"`
}

var handoffDeliveryAuditBeforeLeafOpen = func() {}

func AuditHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (HandoffDeliveryAuditRecord, error) {
	return AuditHandoffDeliveryObservationAt(StateDir(), observation)
}

func AuditHandoffDeliveryObservationAt(stateRoot string, observation issueopscontract.IssueOpsHandoffDeliveryObservation) (HandoffDeliveryAuditRecord, error) {
	auditLogID := auditid.Generate(observation.LifecycleID, observation.AttemptID, []string{observation.PromptSHA256, observation.Launcher.Name})
	observation = redactedHandoffDeliveryObservation(observation)
	observation.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{
		Location: "audit/handoff-delivery.jsonl#audit_log_id=" + auditLogID,
		Digest:   handoffDeliveryReceiptDigest(auditLogID, observation),
	}
	record := HandoffDeliveryAuditRecord{
		OK:            true,
		Kind:          "handoff_delivery_observation",
		SchemaVersion: 1,
		AuditLogID:    auditLogID,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		LogPath:       "audit/handoff-delivery.jsonl",
		Observation:   observation,
	}
	if err := issueopsdomain.ValidateHandoffDeliveryObservation(record.Observation); err != nil {
		return HandoffDeliveryAuditRecord{}, err
	}
	record.RecordDigest = handoffDeliveryRecordDigest(record)
	if err := appendHandoffDeliveryAudit(stateRoot, record); err != nil {
		return record, err
	}
	observations, err := readHandoffDeliveryAuditObservationsAt(stateRoot, observation.LifecycleID, observation.LineageID)
	if err != nil {
		return record, fmt.Errorf("read back handoff delivery audit receipt: %w", err)
	}
	for _, persisted := range observations {
		if persisted.Receipt.Location == record.Observation.Receipt.Location && persisted.Receipt.Digest == record.Observation.Receipt.Digest {
			return record, nil
		}
	}
	return record, errors.New("handoff delivery audit receipt was not readable after append")
}

func ReadHandoffDeliveryAuditObservations() ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	return ReadHandoffDeliveryAuditObservationsAt(StateDir())
}

func ReadHandoffDeliveryAuditObservationsAt(stateRoot string) ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	return readHandoffDeliveryAuditObservationsAt(stateRoot, "", "")
}

func readHandoffDeliveryAuditObservationsAt(stateRoot, lifecycleID, lineageID string) ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	observations := []issueopscontract.IssueOpsHandoffDeliveryObservation{}
	ctx, cancel := context.WithTimeout(context.Background(), processAuditTimeout)
	defer cancel()
	err := WithKeyLock(ctx, stateRoot, "handoff-delivery-audit", func(context.Context) error {
		file, err := openHandoffDeliveryAudit(stateRoot, handoffDeliveryAuditRead)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer file.Close()
		reader := bufio.NewReaderSize(file, 64*1024)
		for {
			line, readErr := reader.ReadBytes('\n')
			if len(line) > 256*1024 {
				return errors.New("handoff delivery audit record exceeds reader limit")
			}
			if errors.Is(readErr, io.EOF) {
				if len(line) == 0 {
					return nil
				}
				// A final frame without the writer's newline was never committed.
				// Preserve the verified prefix and surface the unknown-scope
				// corruption so recovery cannot treat it as evidence of absence.
				return errors.New("handoff delivery audit has a truncated tail")
			}
			if readErr != nil {
				return readErr
			}
			line = line[:len(line)-1]
			var record HandoffDeliveryAuditRecord
			if err := json.Unmarshal(line, &record); err != nil {
				return err
			}
			if !record.OK || record.Kind != "handoff_delivery_observation" || record.SchemaVersion != 1 {
				if handoffDeliveryAuditErrorAffects(record, lifecycleID, lineageID) {
					return errors.New("handoff delivery audit record has invalid kind")
				}
				continue
			}
			if record.RecordDigest == "" || record.RecordDigest != handoffDeliveryRecordDigest(record) {
				if handoffDeliveryAuditErrorAffects(record, lifecycleID, lineageID) {
					return errors.New("handoff delivery audit record digest mismatch")
				}
				continue
			}
			if record.Observation.Receipt.Location != "audit/handoff-delivery.jsonl#audit_log_id="+record.AuditLogID ||
				record.Observation.Receipt.Digest != handoffDeliveryReceiptDigest(record.AuditLogID, record.Observation) {
				if handoffDeliveryAuditErrorAffects(record, lifecycleID, lineageID) {
					return errors.New("handoff delivery audit receipt mismatch")
				}
				continue
			}
			if err := issueopsdomain.ValidateHandoffDeliveryObservation(record.Observation); err != nil {
				if handoffDeliveryAuditErrorAffects(record, lifecycleID, lineageID) {
					return err
				}
				continue
			}
			if lifecycleID != "" && (record.Observation.LifecycleID != lifecycleID || record.Observation.LineageID != lineageID) {
				continue
			}
			observations = append(observations, record.Observation)
		}
	})
	return observations, err
}

func handoffDeliveryAuditErrorAffects(record HandoffDeliveryAuditRecord, lifecycleID, lineageID string) bool {
	if lifecycleID == "" {
		return true
	}
	return record.Observation.LifecycleID == "" || record.Observation.LineageID == "" ||
		(record.Observation.LifecycleID == lifecycleID && record.Observation.LineageID == lineageID)
}

func FoldHandoffDeliveryAuditObservations() (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	return FoldHandoffDeliveryAuditObservationsAt(StateDir())
}

func FoldHandoffDeliveryAuditObservationsAt(stateRoot string) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := ReadHandoffDeliveryAuditObservationsAt(stateRoot)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}

func FoldHandoffDeliveryAuditObservationsForAt(stateRoot, lifecycleID, lineageID string) (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := readHandoffDeliveryAuditObservationsAt(stateRoot, lifecycleID, lineageID)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}

func appendHandoffDeliveryAudit(stateRoot string, record HandoffDeliveryAuditRecord) error {
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(line)+1 > 256*1024 {
		return fmt.Errorf("handoff delivery audit record exceeds reader limit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), processAuditTimeout)
	defer cancel()
	return WithKeyLock(ctx, stateRoot, "handoff-delivery-audit", func(context.Context) error {
		file, err := openHandoffDeliveryAudit(stateRoot, handoffDeliveryAuditAppend)
		if err != nil {
			return err
		}
		defer file.Close()
		written, err := file.Write(append(line, '\n'))
		if err != nil {
			return err
		}
		if written != len(line)+1 {
			return io.ErrShortWrite
		}
		return file.Sync()
	})
}

func handoffDeliveryRecordDigest(record HandoffDeliveryAuditRecord) string {
	record.RecordDigest = ""
	data, _ := json.Marshal(record)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func handoffDeliveryReceiptDigest(auditLogID string, observation issueopscontract.IssueOpsHandoffDeliveryObservation) string {
	observation.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{}
	data, _ := json.Marshal(struct {
		AuditLogID  string                                              `json:"audit_log_id"`
		Observation issueopscontract.IssueOpsHandoffDeliveryObservation `json:"observation"`
	}{AuditLogID: auditLogID, Observation: observation})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func redactedHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) issueopscontract.IssueOpsHandoffDeliveryObservation {
	observation.Launcher.Path = boundedProcessField(policy.RedactFreeform(observation.Launcher.Path))
	if observation.Target.Process != nil {
		process := *observation.Target.Process
		process.Executable = boundedProcessField(policy.RedactFreeform(process.Executable))
		observation.Target.Process = &process
	}
	if observation.OwnerActor != nil && observation.OwnerActor.SessionProcess != nil {
		process := *observation.OwnerActor.SessionProcess
		process.Executable = boundedProcessField(policy.RedactFreeform(process.Executable))
		actor := *observation.OwnerActor
		actor.SessionProcess = &process
		observation.OwnerActor = &actor
	}
	if observation.OwnerClaim.Actor.SessionProcess != nil {
		process := *observation.OwnerClaim.Actor.SessionProcess
		process.Executable = boundedProcessField(policy.RedactFreeform(process.Executable))
		observation.OwnerClaim.Actor.SessionProcess = &process
	}
	return observation
}
