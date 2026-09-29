package issueops

import (
	"context"
	app "issueops/internal/application/issueopsowner"
	model "issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	preparationcontract "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
)

type executionOwnerIssue = model.OwnerIssue
type executionOwnerContextPacket = model.OwnerContextPacket
type executionOwnerCommands = model.OwnerCommands
type executionOwnerSnapshot struct {
	issue                                                             model.OwnerIssue
	requiredDocs, requiredSkills, acceptanceIDs, verificationCommands []string
}
type executionOwnerArtifacts struct{ packetPath, packetSHA256, promptPath, promptSHA256, prompt string }

func ownerContextForTest(root string, read ExecutionIssueSnapshotReadFunc) app.Service {
	return app.Service{Files: OwnerContextFiles{StateRoot: root}, ReadIssue: read, Template: executionOwnerPromptTemplate, ReadRecord: (CycleRecordStore{StateRoot: root}).Load}
}
func ReadExecutionPreparationOwnerEvidence(ctx context.Context, root string, snapshot preparationcontract.Snapshot, read ExecutionIssueSnapshotReadFunc) (preparationcontract.OwnerEvidence, error) {
	return ownerContextForTest(root, read).ReadPreparationEvidence(ctx, snapshot)
}
func PrepareExecutionPreparationOwner(ctx context.Context, root string, snapshot preparationcontract.Snapshot, command preparationcontract.Command, intent preparationcontract.Intent, receipt preparationcontract.IntentReceipt, read ExecutionIssueSnapshotReadFunc) (preparationcontract.OwnerArtifacts, error) {
	return ownerContextForTest(root, read).Prepare(ctx, snapshot, command, intent, receipt)
}
func buildExecutionOwnerArtifacts(record model.IssueOpsRecord, req ExecutionPrepareRequest, snapshot executionOwnerSnapshot, manifest map[string]string) (executionOwnerArtifacts, error) {
	out, err := ownerContextForTest("", nil).Build(record, req, model.OwnerSnapshot{Issue: snapshot.issue, RequiredDocs: snapshot.requiredDocs, RequiredSkills: snapshot.requiredSkills, AcceptanceIDs: snapshot.acceptanceIDs, VerificationCommands: snapshot.verificationCommands}, manifest)
	return executionOwnerArtifacts{packetPath: out.PacketPath, packetSHA256: out.PacketSHA256, promptPath: out.PromptPath, promptSHA256: out.PromptSHA256, prompt: out.Prompt}, err
}
func executionOwnerCommandsFor(record model.IssueOpsRecord, req ExecutionPrepareRequest, digest string) model.OwnerCommands {
	return domain.OwnerCommandsFor(record, req, digest, (OwnerContextFiles{}).Paths(record), app.PolicyContext(record, req))
}
func renderExecutionOwnerPrompt(packet model.OwnerContextPacket, path, digest string) (string, error) {
	return domain.RenderOwnerPrompt(packet, path, digest, executionOwnerPromptTemplate, leasecontract.OwnerArtifactMaxBytes)
}
func validateExecutionOwnerCatalog(commands model.OwnerCommands) error {
	return app.ValidateOwnerCatalog(commands)
}
func RequireStagedExecutionOwnerPlan(root string, record model.IssueOpsRecord) (model.OwnerPlanIdentity, error) {
	return ownerContextForTest(root, nil).RequirePlan(record)
}
func materializeExecutionOwnerArtifacts(root string, record model.IssueOpsRecord) (model.OwnerPlanIdentity, map[string]string, error) {
	return ownerContextForTest(root, nil).MaterializePlan(record)
}
func issueArtifactDirFor(record model.IssueOpsRecord) string { return app.OwnerArtifactDir(record) }
