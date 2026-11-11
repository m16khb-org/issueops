//go:build windows

package osfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ExecutableName returns the file name of an executable called base: Windows
// runs and resolves executables only by a PATHEXT extension.
func ExecutableName(base string) string { return base + ".exe" }

// replaceRetryLimit bounds how long Replace waits for another handle on
// newpath to close.
const replaceRetryLimit = 2 * time.Second

// Replace renames oldpath onto newpath, replacing it atomically. Windows
// refuses to replace a file another process or goroutine briefly holds open
// (a concurrent reader), where Unix rename succeeds; like the Go toolchain's
// robustio, Replace retries only those sharing errors for a bounded time.
func Replace(oldpath, newpath string) error {
	deadline := time.Now().Add(replaceRetryLimit)
	delay := time.Millisecond
	for {
		err := os.Rename(oldpath, newpath)
		if err == nil || !transientSharingError(err) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(delay)
		delay = min(2*delay, 100*time.Millisecond)
	}
}

func transientSharingError(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_SHARING_VIOLATION)
}

// Perm returns a Unix-like permission for path, which info describes.
// Owner bits are rw (rwx for directories and PATHEXT executables), minus w
// when the read-only attribute is set. Group and other bits copy the owner
// bits unless the object is owner-only: owned by the current user, and its
// DACL grants access to no one except that user, SYSTEM, Administrators, and
// OWNER RIGHTS. An unreadable security descriptor counts as not owner-only.
func Perm(path string, info fs.FileInfo) fs.FileMode {
	owner := fs.FileMode(0o600)
	if info.IsDir() || isExecutableName(path) {
		owner = 0o700
	}
	if info.Mode().Perm()&0o200 == 0 {
		owner &^= 0o200
	}
	if private, err := ownerOnly(path); err == nil && private {
		return owner
	}
	return owner | owner>>3 | owner>>6
}

// Chmod sets the read-only attribute from the owner write bit, then gives path
// an owner-only protected DACL when mode grants no group or other access, or
// restores the inherited DACL otherwise. Like chmod, it changes only path.
func Chmod(path string, mode fs.FileMode) error {
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	handle, err := openHandle(path, windows.READ_CONTROL|windows.WRITE_DAC, 0)
	if err != nil {
		return &os.PathError{Op: "chmod", Path: path, Err: err}
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if mode&0o077 != 0 {
		// An empty unprotected DACL leaves only the parent's inheritable ACEs.
		// SetEntriesInAcl returns no ACL for zero entries, so build the 8-byte
		// header directly: revision 2, size 8, no ACEs.
		empty := [2]uint32{2 | 8<<16, 0}
		return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION, nil, nil,
			(*windows.ACL)(unsafe.Pointer(&empty[0])), nil)
	}
	dacl, err := ownerOnlyACL(info.IsDir())
	if err != nil {
		return err
	}
	if err := windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		return &os.PathError{Op: "chmod", Path: path, Err: err}
	}
	return nil
}

// MkdirAll creates path and missing parents. When perm is owner-only, every
// directory it creates gets an owner-only DACL, as Unix mkdir with that perm
// would; existing directories are left unchanged.
func MkdirAll(path string, perm fs.FileMode) error {
	var missing []string
	for dir := filepath.Clean(path); ; {
		if _, err := os.Lstat(dir); err == nil {
			break
		}
		missing = append(missing, dir)
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if err := os.MkdirAll(path, perm); err != nil {
		return err
	}
	if perm&0o077 != 0 {
		return nil
	}
	for _, dir := range slices.Backward(missing) {
		if err := Chmod(dir, perm); err != nil {
			return err
		}
	}
	return nil
}

// Mkdir creates path; an owner-only perm also gives it an owner-only DACL.
func Mkdir(path string, perm fs.FileMode) error {
	if err := os.Mkdir(path, perm); err != nil {
		return err
	}
	if perm&0o077 != 0 {
		return nil
	}
	return Chmod(path, perm)
}

// OpenFile opens name like os.OpenFile. When it creates the file and perm is
// owner-only, the new file gets an owner-only DACL; an existing file keeps
// its ACL, as Unix open keeps an existing file's mode.
func OpenFile(name string, flag int, perm fs.FileMode) (*os.File, error) {
	created := false
	if flag&os.O_CREATE != 0 {
		if _, err := os.Lstat(name); err != nil {
			created = true
		}
	}
	file, err := os.OpenFile(name, flag, perm)
	if err != nil || !created || perm&0o077 != 0 {
		return file, err
	}
	if err := Chmod(name, perm); err != nil {
		_ = file.Close()
		return nil, err
	}
	return file, nil
}

// FileChmod sets the mode of the open file f through its path.
func FileChmod(f *os.File, mode fs.FileMode) error { return Chmod(f.Name(), mode) }

// WriteFile writes data to name like os.WriteFile; a file it creates with an
// owner-only perm gets an owner-only DACL.
func WriteFile(name string, data []byte, perm fs.FileMode) error {
	file, err := OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// PathIdentity returns the identity of the entry at path without following a
// final symlink. info is unused: Windows stat data carries no file index.
func PathIdentity(path string, _ fs.FileInfo) (Identity, bool) {
	handle, err := openHandle(path, windows.FILE_READ_ATTRIBUTES, windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if err != nil {
		return Identity{}, false
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	return handleIdentity(handle)
}

// FileIdentity returns the identity of the open file f.
func FileIdentity(f *os.File, _ fs.FileInfo) (Identity, bool) {
	return handleIdentity(windows.Handle(f.Fd()))
}

func handleIdentity(handle windows.Handle) (Identity, bool) {
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return Identity{}, false
	}
	return Identity{
		Device: uint64(data.VolumeSerialNumber),
		Inode:  uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow),
		Links:  uint64(data.NumberOfLinks),
	}, true
}

// openHandle opens path for metadata access only. Metadata access does not
// conflict with share modes, so it also works on an exclusively held lease.
func openHandle(path string, access, flags uint32) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.InvalidHandle, err
	}
	return windows.CreateFile(name, access,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|flags, 0)
}

func ownerOnly(path string) (bool, error) {
	handle, err := openHandle(path, windows.READ_CONTROL, windows.FILE_FLAG_OPEN_REPARSE_POINT)
	if err != nil {
		return false, err
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, err
	}
	return ownerOnlyDescriptor(sd)
}

func ownerOnlyDescriptor(sd *windows.SECURITY_DESCRIPTOR) (bool, error) {
	principals, err := trustedPrincipals()
	if err != nil {
		return false, err
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(principals.user) {
		return false, err
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		// A NULL DACL grants everyone full access.
		return false, err
	}
	for i := range uint32(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return false, err
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || ace.Header.AceType == windows.ACCESS_DENIED_ACE_TYPE || ace.Mask == 0 {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			// Object, callback, and conditional ACEs are not interpreted; refuse them.
			return false, nil
		}
		if !principals.trusted((*windows.SID)(unsafe.Pointer(&ace.SidStart))) {
			return false, nil
		}
	}
	return true, nil
}

type principalSet struct {
	user    *windows.SID
	trustee []*windows.SID
}

func (p principalSet) trusted(sid *windows.SID) bool {
	return slices.ContainsFunc(p.trustee, sid.Equals)
}

var trustedPrincipals = sync.OnceValues(func() (principalSet, error) {
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return principalSet{}, err
	}
	user, err := tokenUser.User.Sid.Copy()
	if err != nil {
		return principalSet{}, err
	}
	set := principalSet{user: user, trustee: []*windows.SID{user}}
	for _, kind := range []windows.WELL_KNOWN_SID_TYPE{
		windows.WinLocalSystemSid, windows.WinBuiltinAdministratorsSid, windows.WinCreatorOwnerRightsSid,
	} {
		sid, err := windows.CreateWellKnownSid(kind)
		if err != nil {
			return principalSet{}, err
		}
		set.trustee = append(set.trustee, sid)
	}
	return set, nil
})

// ownerOnlyACL grants full control to the current user, SYSTEM, and
// Administrators, the same principals a default user profile grants.
func ownerOnlyACL(directory bool) (*windows.ACL, error) {
	principals, err := trustedPrincipals()
	if err != nil {
		return nil, err
	}
	inheritance := uint32(windows.NO_INHERITANCE)
	if directory {
		inheritance = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	// OWNER RIGHTS is trusted when found but is not granted here. The SIDs are
	// Go memory referenced from TRUSTEE values, so they stay pinned for the call.
	var pinner runtime.Pinner
	defer pinner.Unpin()
	entries := make([]windows.EXPLICIT_ACCESS, 0, 3)
	for _, sid := range principals.trustee[:3] {
		pinner.Pin(sid)
		entries = append(entries, windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inheritance,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		})
	}
	return windows.ACLFromEntries(entries, nil)
}

// isExecutableName reports whether path has a PATHEXT extension, the Windows
// equivalent of an execute bit.
func isExecutableName(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == "" {
		return false
	}
	list := os.Getenv("PATHEXT")
	if list == "" {
		list = ".com;.exe;.bat;.cmd"
	}
	return slices.Contains(filepath.SplitList(strings.ToLower(list)), ext)
}
