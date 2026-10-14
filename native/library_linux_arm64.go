package native

import _ "embed"

const fileName = "edifabric-x12-tools.so"

//go:embed bin/linux-arm64/edifabric-x12-tools.so
var library []byte
