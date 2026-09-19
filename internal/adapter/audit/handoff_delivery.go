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
	"path/filepath"
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

func AuditHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (HandoffDeliveryAuditRecord, error) {
	if err := issueopsdomain.ValidateHandoffDeliveryObservation(observation); err != nil {
		return HandoffDeliveryAuditRecord{}, err
	}
	record := HandoffDeliveryAuditRecord{
		OK:            true,
		Kind:          "handoff_delivery_observation",
		SchemaVersion: 1,
		AuditLogID:    auditid.Generate(observation.LifecycleID, observation.AttemptID, []string{observation.PromptSHA256, observation.Launcher.Name}),
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339Nano),
		LogPath:       "audit/handoff-delivery.jsonl",
		Observation:   redactedHandoffDeliveryObservation(observation),
	}
	if err := issueopsdomain.ValidateHandoffDeliveryObservation(record.Observation); err != nil {
		return HandoffDeliveryAuditRecord{}, err
	}
	record.RecordDigest = handoffDeliveryRecordDigest(record)
	if err := appendHandoffDeliveryAudit(filepath.Join(StateDir(), record.LogPath), record); err != nil {
		return record, err
	}
	return record, nil
}

func ReadHandoffDeliveryAuditObservations() ([]issueopscontract.IssueOpsHandoffDeliveryObservation, error) {
	path := filepath.Join(StateDir(), "audit", "handoff-delivery.jsonl")
	if err := validateHandoffDeliveryAuditFile(path); errors.Is(err, os.ErrNotExist) {
		return []issueopscontract.IssueOpsHandoffDeliveryObservation{}, nil
	} else if err != nil {
		return nil, err
	}
	observations := []issueopscontract.IssueOpsHandoffDeliveryObservation{}
	ctx, cancel := context.WithTimeout(context.Background(), processAuditTimeout)
	defer cancel()
	err := WithKeyLock(ctx, StateDir(), "handoff-delivery-audit", func(context.Context) error {
		file, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer file.Close()
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
		for scanner.Scan() {
			var record HandoffDeliveryAuditRecord
			if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
				return err
			}
			if !record.OK || record.Kind != "handoff_delivery_observation" || record.SchemaVersion != 1 {
				return errors.New("handoff delivery audit record has invalid kind")
			}
			if record.RecordDigest == "" || record.RecordDigest != handoffDeliveryRecordDigest(record) {
				return errors.New("handoff delivery audit record digest mismatch")
			}
			if err := issueopsdomain.ValidateHandoffDeliveryObservation(record.Observation); err != nil {
				return err
			}
			observations = append(observations, record.Observation)
		}
		return scanner.Err()
	})
	return observations, err
}

func FoldHandoffDeliveryAuditObservations() (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := ReadHandoffDeliveryAuditObservations()
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}

func appendHandoffDeliveryAudit(path string, record HandoffDeliveryAuditRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := validateHandoffDeliveryAuditDir(filepath.Dir(path)); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if len(line)+1 > 256*1024 {
		return fmt.Errorf("handoff delivery audit record exceeds reader limit")
	}
	ctx, cancel := context.WithTimeout(context.Background(), processAuditTimeout)
	defer cancel()
	return WithKeyLock(ctx, StateDir(), "handoff-delivery-audit", func(context.Context) error {
		if err := validateHandoffDeliveryAuditFile(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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

func validateHandoffDeliveryAuditFile(path string) error {
	if err := validateHandoffDeliveryAuditDir(filepath.Dir(path)); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("handoff delivery audit log must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return errors.New("handoff delivery audit log must be a regular file")
	}
	if info.Mode().Perm() != 0o600 {
		return errors.New("handoff delivery audit log has unsafe permissions")
	}
	return nil
}

func validateHandoffDeliveryAuditDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return errors.New("handoff delivery audit directory must not be a symlink")
	}
	if !info.Mode().IsDir() {
		return errors.New("handoff delivery audit directory must be a directory")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return errors.New("handoff delivery audit directory has unsafe permissions")
	}
	return nil
}

func redactedHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) issueopscontract.IssueOpsHandoffDeliveryObservation {
	observation.Launcher.Path = boundedProcessField(policy.RedactFreeform(observation.Launcher.Path))
	if observation.Target.Process != nil {
		process := *observation.Target.Process
		process.Executable = boundedProcessField(policy.RedactFreeform(process.Executable))
		observation.Target.Process = &process
	}
	observation.Receipt.Location = boundedProcessField(policy.RedactFreeform(observation.Receipt.Location))
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
