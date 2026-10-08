package port

// Record store bucket names shared by every adapter that reads or writes the
// same rows. Changing a value orphans the stored records.
const (
	// IssueOpsRecordBucket holds the IssueOps cycle records.
	IssueOpsRecordBucket = "issueops_v1"
	// LeaseHolderBucket holds the lease-holder index rows.
	LeaseHolderBucket = "lease_holder_v1"
	// ArtifactStageBucket holds artifacts staged before prepare.
	ArtifactStageBucket = "artifact_stage_v1"
)
