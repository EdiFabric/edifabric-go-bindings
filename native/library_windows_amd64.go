package native

import _ "embed"

const fileName = "edifabric-x12-tools.dll"

//go:embed bin/win-x64/edifabric-x12-tools.dll
var library []byte
