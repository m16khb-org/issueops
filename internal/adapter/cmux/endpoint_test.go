package cmux

import (
	issueops "issueops/internal/contract/issueops"

	"strings"
	"testing"
)

func TestValidateEndpointRejectsOwnerAndIncarnationMismatch(t *testing.T) {
	endpoint := endpointFixture()
	if err := ValidateEndpoint(endpoint, endpoint.Path, int(endpoint.OwnerUID)); err != nil {
		t.Fatal(err)
	}
	wrongOwner := endpoint
	wrongOwner.OwnerUID++
	if err := ValidateEndpoint(wrongOwner, endpoint.Path, int(endpoint.OwnerUID)); err == nil || !strings.Contains(err.Error(), "owner") {
		t.Fatalf("wrong owner error=%v", err)
	}
	changed := endpoint
	changed.Inode++
	if SameEndpoint(endpoint, changed) {
		t.Fatalf("changed endpoint treated as same: before=%+v after=%+v", endpoint, changed)
	}
}

func endpointFixture() issueops.IssueOpsHandoffDeliveryEndpointIncarnation {
	return issueops.IssueOpsHandoffDeliveryEndpointIncarnation{
		Path: "/private/tmp/cmux.sock", Kind: "unix_socket", Device: 1, Inode: 2, CTimeNS: 3,
		OwnerUID: 501, OwnerGID: 20, Mode: 0o600, ParentPath: "/private/tmp", ParentDevice: 1,
		ParentInode: 4, ParentOwnerUID: 0, ParentOwnerGID: 0, ParentMode: 0o1777,
	}
}
