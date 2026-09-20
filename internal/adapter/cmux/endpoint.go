package cmux

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	issueopscontract "issueops/internal/contract/issueops"
)

type EndpointIncarnation = issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation

func ObserveEndpoint(path string, expectedUID int) (EndpointIncarnation, error) {
	if err := requireSupportedPlatform(); err != nil {
		return EndpointIncarnation{}, err
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return EndpointIncarnation{}, fmt.Errorf("cmux socket path must be absolute and clean")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return EndpointIncarnation{}, fmt.Errorf("cmux socket is unavailable: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return EndpointIncarnation{}, fmt.Errorf("cmux socket path must not be a symlink")
	}
	if info.Mode()&os.ModeSocket == 0 {
		return EndpointIncarnation{}, fmt.Errorf("cmux endpoint is not a Unix socket")
	}
	parentPath := filepath.Dir(path)
	parent, err := os.Lstat(parentPath)
	if err != nil {
		return EndpointIncarnation{}, fmt.Errorf("cmux socket parent is unavailable: %w", err)
	}
	if parent.Mode()&os.ModeSymlink != 0 || !parent.IsDir() {
		return EndpointIncarnation{}, fmt.Errorf("cmux socket parent must be a non-symlink directory")
	}
	device, inode, ctimeNS, uid, gid, err := fileIdentity(info)
	if err != nil {
		return EndpointIncarnation{}, err
	}
	parentDevice, parentInode, _, parentUID, parentGID, err := fileIdentity(parent)
	if err != nil {
		return EndpointIncarnation{}, err
	}
	endpoint := EndpointIncarnation{
		Path: path, Kind: "unix_socket", Device: device, Inode: inode, CTimeNS: ctimeNS,
		OwnerUID: uid, OwnerGID: gid, Mode: permissionMode(info.Mode()), ParentPath: parentPath,
		ParentDevice: parentDevice, ParentInode: parentInode, ParentOwnerUID: parentUID,
		ParentOwnerGID: parentGID, ParentMode: permissionMode(parent.Mode()),
	}
	if err := ValidateEndpoint(endpoint, path, expectedUID); err != nil {
		return EndpointIncarnation{}, err
	}
	return endpoint, nil
}

func ValidateEndpoint(endpoint EndpointIncarnation, expectedPath string, expectedUID int) error {
	if endpoint.Path != expectedPath || endpoint.Kind != "unix_socket" || endpoint.Inode == 0 || endpoint.CTimeNS <= 0 || endpoint.ParentInode == 0 {
		return fmt.Errorf("cmux socket endpoint identity is incomplete")
	}
	if int(endpoint.OwnerUID) != expectedUID {
		return fmt.Errorf("cmux socket owner does not match the current uid")
	}
	if endpoint.Mode != 0o600 {
		return fmt.Errorf("cmux socket mode must be 0600")
	}
	if endpoint.ParentOwnerUID != 0 && int(endpoint.ParentOwnerUID) != expectedUID {
		return fmt.Errorf("cmux socket parent owner is unsafe")
	}
	if endpoint.ParentMode&0o022 != 0 && endpoint.ParentMode&0o1000 == 0 {
		return fmt.Errorf("cmux socket parent permissions are unsafe")
	}
	return nil
}

func SameEndpoint(left, right EndpointIncarnation) bool {
	return reflect.DeepEqual(left, right)
}

func permissionMode(mode os.FileMode) uint32 {
	value := uint32(mode.Perm())
	if mode&os.ModeSticky != 0 {
		value |= 0o1000
	}
	return value
}
