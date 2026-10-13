// Package fsutil contains filesystem operations whose durable semantics differ
// between Unix and Windows.
package fsutil

// ReplaceFile atomically publishes source at target, replacing an existing
// target when present.
func ReplaceFile(source, target string) error {
	return replaceFile(source, target)
}

// SyncDir flushes directory metadata where the platform supports it. Windows
// does not permit fsync on directory handles, so its implementation is a no-op.
func SyncDir(path string) error {
	return syncDir(path)
}

// ReadFile reads a file while smoothing over transient sharing conflicts on
// Windows. On Unix it is equivalent to os.ReadFile.
func ReadFile(path string) ([]byte, error) {
	return readFile(path)
}
