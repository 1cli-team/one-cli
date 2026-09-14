//go:build windows && amd64

package mise

import _ "embed"

//go:embed assets/mise-windows-amd64.zip
var bundledArchive []byte
