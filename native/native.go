// Package native embeds the ediFabric Native X12 library for the target
// platform and registers it with the edifabricx12 package. Import it for its
// side effect:
//
//	import _ "github.com/EdiFabric/edifabric-go-bindings/native"
//
// Libraries are embedded for windows/amd64, linux/amd64 and linux/arm64
// (glibc 2.34 or later), and darwin/arm64 (macOS 12 or later). On other
// platforms the import does nothing. Use of the library is governed by the
// EdiFabric End User License Agreement: https://www.edifabric.com/files/eula.pdf
package native

import edifabricx12 "github.com/EdiFabric/edifabric-go-bindings"

func init() {
	if library != nil {
		edifabricx12.RegisterBundledLibrary(fileName, library)
	}
}
