//go:build darwin || freebsd || linux || netbsd

package edifabricx12

import (
	"runtime"

	"github.com/ebitengine/purego"
)

var libraryFile = func() string {
	if runtime.GOOS == "darwin" {
		return "edifabric-x12-tools.dylib"
	}
	return "edifabric-x12-tools.so"
}()

var crtLibraries = func() []string {
	if runtime.GOOS == "darwin" {
		return []string{"/usr/lib/libSystem.B.dylib"}
	}
	return []string{"libc.so.6", "libc.so"}
}()

func openLibrary(path string) (uintptr, error) {
	return purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
}

func lookupSymbol(handle uintptr, name string) (uintptr, error) {
	return purego.Dlsym(handle, name)
}
