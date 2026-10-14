package native

import _ "embed"

const fileName = "edifabric-x12-tools.dylib"

//go:embed bin/osx-arm64/edifabric-x12-tools.dylib
var library []byte
