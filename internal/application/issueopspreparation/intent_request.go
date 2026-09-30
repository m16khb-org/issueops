package issueopspreparation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	preparationcontract "issueops/internal/contract/issueopspreparation"
	preparationdomain "issueops/internal/domain/issueopspreparation"
)

type IntentFiles interface {
	SamePath(string, string) bool
	ArtifactPaths(preparationcontract.Record) (string, string)
	ReadToken(preparationcontract.Record) (string, error)
	ReadArtifact(string, string) ([]byte, error)
}

type IntentRequestBuilder struct{ Files IntentFiles }

func (s IntentRequestBuilder) Build(record preparationcontract.Record, payload preparationcontract.Intent) (preparationcontract.IntentRequest, error) {
	if err := preparationdomain.ValidateIntentRecordAuthority(record, payload); err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	request, err := s.Inspect(record, payload)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if payload.Launch == nil {
		return request, nil
	}
	token, err := s.Files.ReadToken(record)
	if err != nil || intentDigest([]byte(token)) != payload.ClaimTokenSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed claim token identity changed")
	}
	prompt, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, payload.Launch.PromptPath)
	if err != nil || intentDigest(prompt) != payload.Launch.PromptSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	packet, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, payload.Launch.ContextPacketPath)
	if err != nil || intentDigest(packet) != payload.Launch.ContextPacketSHA256 {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed context packet identity changed")
	}
	request.Launch.Prompt = string(prompt)
	return request, nil
}

// Inspect uses sealed metadata without opening token or owner artifact files.
// Cleanup must still inspect external resources after a worktree has disappeared.
func (s IntentRequestBuilder) Inspect(record preparationcontract.Record, payload preparationcontract.Intent) (preparationcontract.IntentRequest, error) {
	if err := (preparationcontract.IntentCodec{}).Validate(payload, payload.OperationID); err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if err := preparationdomain.ValidateIntentRecordIdentity(record, payload, s.paths(record, payload)); err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	request := preparationcontract.IntentRequest{
		Stage: payload.Stage, OperationID: payload.OperationID,
		OrcaRequestID: payload.OrcaRequestID, OrcaPromptRequestID: payload.OrcaPromptRequestID, Generation: payload.Generation, Marker: payload.Marker,
		Workspace: payload.Workspace, Probe: payload.Probe,
		Prepared: payload.Prepared, TerminalPTYID: payload.TerminalPTYID,
		RunID: payload.RunID, RunBound: payload.RunBound, TaskID: payload.TaskID,
	}
	if payload.Launch != nil {
		expectedPacketPath, expectedPromptPath := s.Files.ArtifactPaths(record)
		if !s.Files.SamePath(payload.Launch.PromptPath, expectedPromptPath) || !s.Files.SamePath(payload.Launch.ContextPacketPath, expectedPacketPath) {
			return preparationcontract.IntentRequest{}, fmt.Errorf("sealed owner artifact path changed")
		}
		request.Launch = &preparationcontract.LaunchRequest{
			PromptPath: payload.Launch.PromptPath, PromptSHA256: payload.Launch.PromptSHA256,
			ContextPacketPath: payload.Launch.ContextPacketPath, ContextPacketSHA256: payload.Launch.ContextPacketSHA256,
		}
	}
	return request, nil
}

func (s IntentRequestBuilder) paths(record preparationcontract.Record, payload preparationcontract.Intent) preparationdomain.IntentIdentityPaths {
	var observed preparationdomain.IntentIdentityPaths
	if record.Execution == nil {
		return observed
	}
	workspace := record.Execution.Workspace
	observed.Source = s.Files.SamePath(workspace.SourceRoot, payload.Workspace.SourceRoot)
	observed.Root = s.Files.SamePath(workspace.Root, payload.Workspace.Root)
	observed.Parent = s.optionalPath(workspace.ParentWorktree, payload.Workspace.ParentWorktree)
	if payload.Prepared != nil {
		observed.PreparedRecordedRoot = s.Files.SamePath(record.WorktreePath, payload.Prepared.Workspace.Root)
		observed.PreparedRoot = s.Files.SamePath(workspace.Root, payload.Prepared.Workspace.Root)
		observed.PreparedParent = s.optionalPath(workspace.ParentWorktree, payload.Prepared.Workspace.ParentWorktree)
	}
	return observed
}

func (s IntentRequestBuilder) optionalPath(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return a == b
	}
	return s.Files.SamePath(a, b)
}

func intentDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// LaunchHydrator re-reads the sealed launch before materializing its prompt.
type LaunchHydrator struct {
	ReadRecord func(string) (preparationcontract.Record, error)
	ReadIntent func(string) (preparationcontract.Intent, error)
	Builder    IntentRequestBuilder
}

func (s LaunchHydrator) Hydrate(id string, request preparationcontract.IntentRequest) (preparationcontract.IntentRequest, error) {
	record, err := s.ReadRecord(id)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if record.Execution == nil || record.Execution.Pending == nil {
		return preparationcontract.IntentRequest{}, fmt.Errorf("sealed Orca intent is unavailable")
	}
	intent, err := s.ReadIntent(record.Execution.Pending.OperationID)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	if err := preparationdomain.ValidateLaunchRequest(request, intent); err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	hydrated, err := s.Builder.Build(record, intent)
	if err != nil {
		return preparationcontract.IntentRequest{}, err
	}
	launch := *request.Launch
	launch.Prompt = hydrated.Launch.Prompt
	request.Launch = &launch
	return request, nil
}
