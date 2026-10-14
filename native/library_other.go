//go:build !(windows && amd64) && !(linux && amd64) && !(linux && arm64) && !(darwin && arm64)

package native

const fileName = ""

var library []byte
