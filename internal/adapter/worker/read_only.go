package worker

import (
	policycontract "issueops/internal/contract/policy"
	workercontract "issueops/internal/contract/worker"
)

func RunReadOnlyWorkerJob(kind, payload string, request policycontract.CommandPolicyRequest) (workercontract.WorkerJob, error) {
	return workerService().RunReadOnly(kind, payload, request)
}
