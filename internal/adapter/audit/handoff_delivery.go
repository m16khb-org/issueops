package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/domain/auditid"
	issueopsdomain "issueops/internal/domain/issueops"
	"issueops/internal/domain/policy"
)

type HandoffDeliveryAuditRecord struct {
	OK          bool                                                `json:"ok"`
	Kind        string                                              `json:"kind"`
	AuditLogID  string                                              `json:"audit_log_id"`
	GeneratedAt string                                              `json:"generated_at"`
	LogPath     string                                              `json:"log_path,omitempty"`
	Observation issueopscontract.IssueOpsHandoffDeliveryObservation `json:"observation"`
}

func AuditHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) (HandoffDeliveryAuditRecord, error) {
	if err := issueopsdomain.ValidateHandoffDeliveryObservation(observation); err != nil {
		return HandoffDeliveryAuditRecord{}, err
	}
	record := HandoffDeliveryAuditRecord{
		OK:          true,
		Kind:        "handoff_delivery_observation",
		AuditLogID:  auditid.Generate(observation.LifecycleID, observation.AttemptID, []string{observation.PromptSHA256, observation.Launcher.Name}),
		GeneratedAt: time.Now().UTC().Format(time.RFC3339Nano),
		LogPath:     filepath.Join(StateDir(), "audit", "handoff-delivery.jsonl"),
		Observation: redactedHandoffDeliveryObservation(observation),
	}
	if err := appendHandoffDeliveryAudit(record.LogPath, record); err != nil {
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
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return []issueopscontract.IssueOpsHandoffDeliveryObservation{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	observations := []issueopscontract.IssueOpsHandoffDeliveryObservation{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)
	for scanner.Scan() {
		var record HandoffDeliveryAuditRecord
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			return nil, err
		}
		if !record.OK || record.Kind != "handoff_delivery_observation" {
			return nil, errors.New("handoff delivery audit record has invalid kind")
		}
		if err := issueopsdomain.ValidateHandoffDeliveryObservation(record.Observation); err != nil {
			return nil, err
		}
		observations = append(observations, record.Observation)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return observations, nil
}

func FoldHandoffDeliveryAuditObservations() (map[string]issueopscontract.IssueOpsHandoffDeliveryObservation, []issueopscontract.IssueOpsHandoffDeliveryDecision, error) {
	observations, err := ReadHandoffDeliveryAuditObservations()
	if err != nil {
		return nil, nil, err
	}
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, nil
}

func appendHandoffDeliveryAudit(path string, record HandoffDeliveryAuditRecord) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	line, err := json.Marshal(record)
	if err != nil {
		return err
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
		_, err = file.Write(append(line, '\n'))
		return err
	})
}

func validateHandoffDeliveryAuditFile(path string) error {
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

func redactedHandoffDeliveryObservation(observation issueopscontract.IssueOpsHandoffDeliveryObservation) issueopscontract.IssueOpsHandoffDeliveryObservation {
	observation.Launcher.Path = boundedProcessField(policy.RedactFreeform(observation.Launcher.Path))
	observation.Target.Process.Executable = boundedProcessField(policy.RedactFreeform(observation.Target.Process.Executable))
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
