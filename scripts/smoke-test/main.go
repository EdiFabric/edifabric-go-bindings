// Command smoke-test checks that the native package's embedded library loads.
// Build it, then run it from a folder with no copy of the library.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	edifabricx12 "github.com/EdiFabric/edifabric-go-bindings"
	_ "github.com/EdiFabric/edifabric-go-bindings/native"
)

func main() {
	lib, err := edifabricx12.LoadLibrary("")
	if err != nil {
		fail(err)
	}
	fmt.Println("loaded", lib.Path)
	if !strings.Contains(lib.Path, string(filepath.Separator)+"edifabric-x12"+string(filepath.Separator)) {
		fail(fmt.Errorf("expected the library extracted from the native package, got %s", lib.Path))
	}
	version, err := edifabricx12.GetAppVersion()
	if err != nil {
		fail(err)
	}
	fmt.Println("GetAppVersion ->", version)

	// The license check uses OpenSSL on Linux, which loading alone doesn't.
	serial := os.Getenv("EDIFABRIC_SERIAL")
	if serial == "" {
		serial = "bd96a836feca45cb91c86ee65d281f52"
	}
	if err := edifabricx12.EnsureToken(serial, 3600); err != nil {
		fail(err)
	}
	fmt.Println("EnsureToken -> ok")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
