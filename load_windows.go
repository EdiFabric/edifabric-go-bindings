//go:build windows

package edifabricx12

import "syscall"

const libraryFile = "edifabric-x12-tools.dll"

var crtLibraries = []string{"ucrtbase.dll", "msvcrt.dll"}

func openLibrary(path string) (uintptr, error) {
	handle, err := syscall.LoadLibrary(path)
	return uintptr(handle), err
}

func lookupSymbol(handle uintptr, name string) (uintptr, error) {
	return syscall.GetProcAddress(syscall.Handle(handle), name)
}
