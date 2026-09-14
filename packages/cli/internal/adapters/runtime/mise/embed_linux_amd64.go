//go:build linux && amd64

package mise

import _ "embed"

//go:embed assets/mise-linux-amd64.tar.gz
var bundledArchive []byte
