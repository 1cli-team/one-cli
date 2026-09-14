//go:build darwin && arm64

package mise

import _ "embed"

//go:embed assets/mise-darwin-arm64.tar.gz
var bundledArchive []byte
