// Package osfile reports file permission and identity facts in one shape on
// every supported OS.
//
// Unix returns the native mode bits and stat identity unchanged. Windows has
// no mode bits: Go reports 0666 for every writable file and 0777 for every
// directory, so owner-only checks written against Unix modes would reject
// every file. Windows instead derives the same Unix-like permission from the
// object's owner SID and DACL, and identity from the volume serial and file
// index. Callers keep one owner-only check that is meaningful on both.
package osfile

// Identity is the physical file identity: device and inode on Unix, volume
// serial and file index on Windows.
type Identity struct {
	Device uint64
	Inode  uint64
	Links  uint64
}
