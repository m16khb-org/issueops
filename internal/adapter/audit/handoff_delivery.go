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

var (
	handoffDeliveryAuditBeforeStateRootOpen = func() {}
	handoffDeliveryAuditBeforeLeafOpen      = func() {}
	handoffDeliveryAuditAfterLeafOpen       = func() {}
)

type handoffDeliveryAuditHandle struct {
	file       *os.File
	verifyPath func() error
	closePath  func() error
}

func (handle *handoffDeliveryAuditHandle) Close() error {
	if handle == nil {
		return nil
	}
	var errs []error
	if handle.file != nil {
		errs = append(errs, handle.file.Close())
	}
	if handle.closePath != nil {
		errs = append(errs, handle.closePath())
	}
	return errors.Join(errs...)
}

func (handle *handoffDeliveryAuditHandle) VerifyPath() error {
	if handle == nil || handle.verifyPath == nil {
		return errors.New("handoff delivery audit path verifier is unavailable")
	}
	return handle.verifyPath()
}

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
	return record, nil
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
		handle, err := openHandoffDeliveryAudit(stateRoot, handoffDeliveryAuditRead)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer handle.Close()
		readErr := scanHandoffDeliveryAudit(handle.file, lifecycleID, lineageID, func(observation issueopscontract.IssueOpsHandoffDeliveryObservation) {
			observations = append(observations, observation)
		})
		return errors.Join(readErr, handle.VerifyPath())
	})
	return observations, err
}

func scanHandoffDeliveryAudit(file *os.File, lifecycleID, lineageID string, accept func(issueopscontract.IssueOpsHandoffDeliveryObservation)) error {
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
		accept(record.Observation)
	}
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
		handle, err := openHandoffDeliveryAudit(stateRoot, handoffDeliveryAuditAppend)
		if err != nil {
			return err
		}
		defer handle.Close()
		written, err := handle.file.Write(append(line, '\n'))
		if err != nil {
			return err
		}
		if written != len(line)+1 {
			return io.ErrShortWrite
		}
		if err := handle.file.Sync(); err != nil {
			return err
		}
		if _, err := handle.file.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seek handoff delivery audit receipt: %w", err)
		}
		found := false
		readErr := scanHandoffDeliveryAudit(handle.file, record.Observation.LifecycleID, record.Observation.LineageID, func(observation issueopscontract.IssueOpsHandoffDeliveryObservation) {
			if observation.Receipt.Location == record.Observation.Receipt.Location && observation.Receipt.Digest == record.Observation.Receipt.Digest {
				found = true
			}
		})
		if err := errors.Join(readErr, handle.VerifyPath()); err != nil {
			return fmt.Errorf("read back handoff delivery audit receipt: %w", err)
		}
		if !found {
			return errors.New("handoff delivery audit receipt was not readable after append")
		}
		return nil
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
