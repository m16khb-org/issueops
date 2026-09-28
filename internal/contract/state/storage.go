package state

// The record writer lease is a stable store file shared by SQL writers and
// exclusive cleanup. Its name is SHA-256(RecordWriteLeaseKey) plus .lock.
const (
	RecordWriteLeaseKey  = "sqlstore-record-writes"
	RecordWriteLeaseFile = "ad84770f6bed5eba9ced2670c0b244434eaa7501c5be094ade7d034627f06437.lock"
)
