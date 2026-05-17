// Dev-only module declaration. This file isolates the go-lib template's
// literal *.go files from the parent one-cli Go module during repository
// development, so `go build ./...` at the repo root doesn't try to compile
// these template fragments. The renderer skips this file when materialising
// the template; the user's actual go.mod is rendered from go.mod.hbs.
module template-go-lib

go 1.23
