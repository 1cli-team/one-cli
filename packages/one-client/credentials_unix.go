//go:build !windows

package oneclient

import (
	"os"
	"syscall"
)

func openCredentialFile(path string) (*os.File, error) {
	// Do not follow a replaced symlink or block on an unexpected FIFO.
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
}
func privateCredentialOwner(info os.FileInfo, directory bool) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Uid != uint32(os.Geteuid()) {
		return false
	}
	if directory {
		return info.Mode().Perm()&0022 == 0
	}
	return info.Mode().Perm() == 0600
}
