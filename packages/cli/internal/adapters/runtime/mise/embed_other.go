//go:build (!linux && !darwin && !windows) || (!amd64 && !arm64) || (windows && arm64)

package mise

var bundledArchive []byte
