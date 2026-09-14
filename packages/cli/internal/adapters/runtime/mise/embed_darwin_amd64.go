//go:build darwin && amd64

package mise

import _ "embed"

//go:embed assets/mise-darwin-amd64.tar.gz
var bundledArchive []byte
