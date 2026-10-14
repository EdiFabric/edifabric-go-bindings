package edifabricx12

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

var bundled struct {
	name string
	data []byte
}

// RegisterBundledLibrary registers a native library embedded in the program.
// LoadLibrary writes it to the user cache folder and loads it from there when
// no other library is found. Importing
// github.com/EdiFabric/edifabric-go-bindings/native registers the library for
// the target platform, so most programs never call this directly.
func RegisterBundledLibrary(fileName string, data []byte) {
	mu.Lock()
	defer mu.Unlock()
	bundled.name, bundled.data = fileName, data
}

// extractBundled writes the registered library to
// <user cache>/edifabric-x12/<hash>/, because the OS can only load it from a
// file. Later runs reuse the copy.
func extractBundled() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	sum := sha256.Sum256(bundled.data)
	dir := filepath.Join(base, "edifabric-x12", hex.EncodeToString(sum[:8]))
	target := filepath.Join(dir, bundled.name)
	if info, err := os.Stat(target); err == nil && info.Size() == int64(len(bundled.data)) {
		return target, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	partial, err := os.CreateTemp(dir, bundled.name+".*.part")
	if err != nil {
		return "", err
	}
	defer os.Remove(partial.Name())
	_, err = partial.Write(bundled.data)
	if closeErr := partial.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return "", err
	}
	if err := os.Rename(partial.Name(), target); err != nil {
		// Another process may have extracted (and on Windows locked) the same file.
		if info, statErr := os.Stat(target); statErr != nil || info.Size() != int64(len(bundled.data)) {
			return "", err
		}
	}
	return target, nil
}
