//go:build linux && arm64

package mise

import _ "embed"

//go:embed assets/mise-linux-arm64.tar.gz
var bundledArchive []byte
