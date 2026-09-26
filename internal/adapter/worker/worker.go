package worker

import workercontract "issueops/internal/contract/worker"

func EnqueueWorkerJob(kind, payload string) (workercontract.WorkerJob, error) {
	return workerService().Enqueue(kind, payload)
}

func CancelWorkerJob(id string) (workercontract.WorkerJob, error) {
	return workerService().Cancel(id)
}
