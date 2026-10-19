package oneclient

import "os"

func openCredentialFile(path string) (*os.File, error) { return os.Open(path) }

// Windows uses directory ACLs; POSIX mode bits are not an access-control check.
func privateCredentialOwner(_ os.FileInfo, _ bool) bool { return true }
