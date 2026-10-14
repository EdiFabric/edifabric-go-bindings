// Package edifabricx12 provides Go bindings for ediFabric Native X12 (see c-abi-edifabric_x12_tools.h).
//
// Load the platform shared library (edifabric-x12-tools.dll / .so / .dylib) and call
// the wrappers below. All payloads cross the boundary as UTF-8 byte buffers.
// Native return codes: 0 = success, 1 = InsufficientCapacity (grow-and-retry),
// anything else is an error whose message comes from GetError.
//
// The wrappers handle grow-and-retry internally and return *Error on failure.
// The library is loaded with purego, so cgo and a C compiler are not required.
package edifabricx12

import (
	"errors"
	"fmt"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// ParseMode is the mode argument for Parse and StartSplit.
type ParseMode int32

const (
	ParseJSON            ParseMode = 1
	ParseJSONValidate    ParseMode = 2
	ParseJSONValidateAck ParseMode = 3
)

// LogLevel is the minLevel argument for InitLogger.
type LogLevel int32

const (
	LogTrace       LogLevel = 0
	LogDebug       LogLevel = 1
	LogInformation LogLevel = 2
	LogWarning     LogLevel = 3
	LogError       LogLevel = 4
	LogAll         LogLevel = 5
)

// Library-level error codes. Validation codes come from the engine.
const (
	CodeSuccess                 = 0
	CodeInsufficientCapacity    = 1
	CodeUnknown                 = 501
	CodeNoConnection            = 502
	CodeModelMapInvalid         = 503
	CodeIncorrectInput          = 611
	CodeLoggerInitialization    = 612
	CodeMapDeserialization      = 613
	CodeIncorrectCapacity       = 614
	CodeMapNotSet               = 615
	CodeIncorrectMode           = 616
	CodeNoJSON                  = 617
	CodeValidationUnavailable   = 618
	CodeValidationSerialization = 619
	CodeIncorrectToken          = 620
	CodeConfigDeserialization   = 621
	CodeSplitSegmentIDMissing   = 622
	CodeSplitNotStarted         = 623
	CodeNoResult                = 624
	CodeResultSizeMismatch      = 625
	CodeMergeNotStarted         = 626
	CodeIncorrectOutputPointer  = 627
	CodeIncorrectSerial         = 628
	CodeLicenseNotInstalled     = 629
	CodeAppVersionExceeded      = 630
	CodeTokenExpired            = 631
	CodeTokenMissing            = 632
	CodeMaxLicensesExceeded     = 633
	CodeLicenseSnapshotMissing  = 634
	CodeLicenseNotSet           = 635
	CodeRateExceeded            = 636
	CodeInvalidJSON             = 637
	CodeIncorrectLicense        = 638
	CodeLicenseQuotaExceeded    = 639
)

// Error is returned when a native call returns a non-zero status code.
type Error struct {
	Code    int
	Message string
	API     string
}

func (e *Error) Error() string {
	if e.API != "" {
		return fmt.Sprintf("%s: error %d: %s", e.API, e.Code, e.Message)
	}
	return fmt.Sprintf("error %d: %s", e.Code, e.Message)
}

// Library describes the loaded native library.
type Library struct {
	Path   string
	Handle uintptr
}

// ---------------------------------------------------------------------------
// Native functions
// ---------------------------------------------------------------------------

var (
	cInitLogger         func(path *byte, pathLen int32, minLevel int32) int32
	cShutdownLogger     func() int32
	cClearCache         func() int32
	cEnsureToken        func(serial *byte, serialLen int32, seconds int32) int32
	cGetAppVersion      func(appVersion *int32) int32
	cGetToken           func(serial *byte, serialLen int32, output *byte, outputCapacity int32, outputLength *int32) int32
	cValidateToken      func(token *byte, tokenLen int32) int32
	cSetToken           func(token *byte, tokenLen int32) int32
	cGetTokenExpiration func(expirationUTC *int64) int32
	cSetSerial          func(serial *byte, serialLen int32) int32
	cSetMap             func(mapJSON *byte, mapLength int32) int32
	cParse              func(input *byte, inputLength int32, mode int32, config *byte, configLength int32,
		output *byte, outputCapacity int32, outputLength *int32, outputOffset *int32) int32
	cStartSplit  func(input *byte, inputLength int32, mode int32, config *byte, configLength int32) int32
	cSplit       func(resultSize *int32, resultOffset *int32, last *uint8) int32
	cBuild       func(input *byte, inputLength int32, postfix *byte, output *byte, outputCapacity int32, outputLength *int32) int32
	cStartMerge  func(input *byte, inputLength int32) int32
	cMerge       func(resultSize *int32) int32
	cGetResult   func(buffer *byte, bufferSize int32) int32
	cGetError    func(errorCode int32) *byte
	cFreeError   func(ptr *byte)
	cCrtFree     func(ptr *byte)
	crtFreeTried bool
)

var (
	mu      sync.Mutex
	current *Library
)

// ---------------------------------------------------------------------------
// Library loading
// ---------------------------------------------------------------------------

// LibraryFilename returns the native library file name for this platform.
func LibraryFilename() string { return libraryFile }

func resolveCandidate(path string) string {
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		return filepath.Join(path, libraryFile)
	}
	return path
}

func candidates(explicit string) []string {
	var found []string
	if explicit != "" {
		found = append(found, resolveCandidate(explicit))
	}
	if env := os.Getenv("EDIFABRIC_X12_LIB"); env != "" {
		found = append(found, resolveCandidate(env))
	}
	if exe, err := os.Executable(); err == nil {
		found = append(found, filepath.Join(filepath.Dir(exe), libraryFile))
	}
	if wd, err := os.Getwd(); err == nil {
		found = append(found, filepath.Join(wd, libraryFile), filepath.Join(filepath.Dir(wd), libraryFile))
	}
	return found
}

func bind(path string) (*Library, error) {
	handle, err := openLibrary(path)
	if err != nil {
		return nil, err
	}
	functions := []struct {
		name string
		fptr any
	}{
		{"init_logger", &cInitLogger},
		{"shutdown_logger", &cShutdownLogger},
		{"clear_cache", &cClearCache},
		{"ensure_token", &cEnsureToken},
		{"get_app_version", &cGetAppVersion},
		{"get_token", &cGetToken},
		{"validate_token", &cValidateToken},
		{"set_token", &cSetToken},
		{"get_token_expiration", &cGetTokenExpiration},
		{"set_serial", &cSetSerial},
		{"set_map", &cSetMap},
		{"parse", &cParse},
		{"start_split", &cStartSplit},
		{"split", &cSplit},
		{"build", &cBuild},
		{"start_merge", &cStartMerge},
		{"merge", &cMerge},
		{"get_result", &cGetResult},
		{"get_error", &cGetError},
	}
	for _, f := range functions {
		symbol, err := lookupSymbol(handle, f.name)
		if err != nil || symbol == 0 {
			return nil, fmt.Errorf("cannot find function %q in %s", f.name, path)
		}
		purego.RegisterFunc(f.fptr, symbol)
	}
	cFreeError = nil
	if symbol, err := lookupSymbol(handle, "free_error"); err == nil && symbol != 0 {
		purego.RegisterFunc(&cFreeError, symbol)
	}
	return &Library{Path: path, Handle: handle}, nil
}

// LoadLibrary loads and binds the native library.
//
// path may be the library file or the folder containing it. When empty, the
// EDIFABRIC_X12_LIB variable, the executable's folder, the working directory,
// and its parent are tried in that order, then the library registered with
// RegisterBundledLibrary. Cached after the first success.
func LoadLibrary(path string) (*Library, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadLocked(path)
}

func loadLocked(path string) (*Library, error) {
	if current != nil && path == "" {
		return current, nil
	}
	tried := candidates(path)
	var lastErr error
	for _, candidate := range tried {
		if info, err := os.Stat(candidate); err != nil || info.IsDir() {
			continue
		}
		lib, err := bind(candidate)
		if err != nil {
			lastErr = err
			continue
		}
		current = lib
		return lib, nil
	}
	if bundled.data != nil {
		tried = append(tried, "the bundled library")
		file, err := extractBundled()
		if err == nil {
			var lib *Library
			if lib, err = bind(file); err == nil {
				current = lib
				return lib, nil
			}
		}
		lastErr = err
	}
	return nil, fmt.Errorf("could not load %s. Tried: %s. Last error: %v",
		libraryFile, strings.Join(tried, ", "), lastErr)
}

func ensure() error {
	mu.Lock()
	defer mu.Unlock()
	_, err := loadLocked("")
	return err
}

// ---------------------------------------------------------------------------
// Buffer / error helpers
// ---------------------------------------------------------------------------

// buffer returns a pointer and length; nil and 0 for empty input so the ABI receives a null pointer.
func buffer(data []byte) (*byte, int32) {
	if len(data) == 0 {
		return nil, 0
	}
	return &data[0], int32(len(data))
}

// DecodeString copies a NUL-terminated native string.
func DecodeString(ptr *byte) string {
	if ptr == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(ptr), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(ptr, n))
}

func crtFree() func(*byte) {
	mu.Lock()
	defer mu.Unlock()
	if !crtFreeTried {
		crtFreeTried = true
		for _, name := range crtLibraries {
			handle, err := openLibrary(name)
			if err != nil {
				continue
			}
			if symbol, err := lookupSymbol(handle, "free"); err == nil && symbol != 0 {
				purego.RegisterFunc(&cCrtFree, symbol)
				break
			}
		}
	}
	return cCrtFree
}

// RawGetError calls the raw get_error export. The caller owns the returned
// string and must release it with FreeError.
func RawGetError(errorCode int) (*byte, error) {
	if err := ensure(); err != nil {
		return nil, err
	}
	return cGetError(int32(errorCode)), nil
}

// FreeError releases a string returned by RawGetError.
//
// GetError already does this; call FreeError only when you invoke the raw
// export yourself. Builds that do not export free_error fall back to the C runtime free.
func FreeError(ptr *byte) {
	if ptr == nil {
		return
	}
	if cFreeError != nil {
		cFreeError(ptr)
	} else if free := crtFree(); free != nil {
		free(ptr)
	}
}

// GetError returns the message for an error code, releasing the native string.
func GetError(errorCode int) string {
	ptr, err := RawGetError(errorCode)
	if err != nil {
		return err.Error()
	}
	if ptr == nil {
		return fmt.Sprintf("Unknown error %d", errorCode)
	}
	defer FreeError(ptr)
	if message := DecodeString(ptr); message != "" {
		return message
	}
	return fmt.Sprintf("Unknown error %d", errorCode)
}

// Check returns *Error unless rc is CodeSuccess.
func Check(rc int32, api string) error {
	if rc == CodeSuccess {
		return nil
	}
	return &Error{Code: int(rc), Message: GetError(int(rc)), API: api}
}

// withGrowth runs a grow-and-retry output call.
func withGrowth(capacity int, api string, call func(output []byte, outputLength *int32) int32) ([]byte, error) {
	capacity = max(1, capacity)
	for {
		output := make([]byte, capacity)
		var outputLength int32
		rc := call(output, &outputLength)
		if rc == CodeInsufficientCapacity {
			capacity = max(int(outputLength), capacity*2)
			continue
		}
		if err := Check(rc, api); err != nil {
			return nil, err
		}
		return output[:outputLength], nil
	}
}

// ---------------------------------------------------------------------------
// Lifecycle and logging
// ---------------------------------------------------------------------------

// InitLogger writes a log file at path; minLevel is LogTrace through LogError.
func InitLogger(path string, minLevel LogLevel) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer([]byte(path))
	return Check(cInitLogger(ptr, length, int32(minLevel)), "init_logger")
}

// ShutdownLogger flushes and stops the logger.
func ShutdownLogger() error {
	if err := ensure(); err != nil {
		return err
	}
	return Check(cShutdownLogger(), "shutdown_logger")
}

// ClearCache resets the model map, split/merge state, last result, license, and logger.
func ClearCache() error {
	if err := ensure(); err != nil {
		return err
	}
	return Check(cClearCache(), "clear_cache")
}

// ---------------------------------------------------------------------------
// Licensing
// ---------------------------------------------------------------------------

// EnsureToken caches a token for runtime authorization against the license server.
//
// It retrieves a new token if the cached token expires within the specified
// number of seconds.
func EnsureToken(serial string, seconds int) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer([]byte(serial))
	return Check(cEnsureToken(ptr, length, int32(seconds)), "ensure_token")
}

// GetAppVersion returns the library application version.
func GetAppVersion() (int, error) {
	if err := ensure(); err != nil {
		return 0, err
	}
	var version int32
	if err := Check(cGetAppVersion(&version), "get_app_version"); err != nil {
		return 0, err
	}
	return int(version), nil
}

// GetToken fetches a signed license token for the serial. Requires internet access.
func GetToken(serial string) (string, error) {
	if err := ensure(); err != nil {
		return "", err
	}
	serialPtr, serialLength := buffer([]byte(serial))
	token, err := withGrowth(4096, "get_token", func(output []byte, outputLength *int32) int32 {
		return cGetToken(serialPtr, serialLength, &output[0], int32(len(output)), outputLength)
	})
	return string(token), err
}

// ValidateToken validates a token without caching it.
func ValidateToken(token string) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer([]byte(token))
	return Check(cValidateToken(ptr, length), "validate_token")
}

// SetToken caches a token for this process.
func SetToken(token string) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer([]byte(token))
	return Check(cSetToken(ptr, length), "set_token")
}

// GetTokenExpiration returns the token expiry as .NET UTC ticks; 0 when no token is set.
func GetTokenExpiration() (int64, error) {
	if err := ensure(); err != nil {
		return 0, err
	}
	var expiration int64
	if err := Check(cGetTokenExpiration(&expiration), "get_token_expiration"); err != nil {
		return 0, err
	}
	return expiration, nil
}

// TicksToTime converts .NET UTC ticks (100 ns intervals since year 1) to time.Time.
func TicksToTime(ticks int64) time.Time {
	const ticksPerSecond = 10_000_000
	const unixEpochSeconds = 62_135_596_800
	return time.Unix(ticks/ticksPerSecond-unixEpochSeconds, (ticks%ticksPerSecond)*100).UTC()
}

// SetSerial caches a serial for runtime authorization against the license server.
func SetSerial(serial string) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer([]byte(serial))
	return Check(cSetSerial(ptr, length), "set_serial")
}

// ---------------------------------------------------------------------------
// Model map
// ---------------------------------------------------------------------------

// SetMap loads the template map JSON. Call once before Parse or StartSplit.
func SetMap(mapJSON []byte) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer(mapJSON)
	return Check(cSetMap(ptr, length), "set_map")
}

// ---------------------------------------------------------------------------
// Parse, split, build, merge
// ---------------------------------------------------------------------------

// Parse parses a whole interchange and returns the output and offset.
//
// offset is 0 for ParseJSON; for ParseJSONValidate and ParseJSONValidateAck it
// marks where the validation and acknowledgment section begins inside output.
// config may be nil.
func Parse(edi []byte, mode ParseMode, config []byte) (output []byte, offset int, err error) {
	if err := ensure(); err != nil {
		return nil, 0, err
	}
	inputPtr, inputLength := buffer(edi)
	configPtr, configLength := buffer(config)
	capacity := max(4096, len(edi)*12)
	for {
		buf := make([]byte, capacity)
		var outputLength, outputOffset int32
		rc := cParse(inputPtr, inputLength, int32(mode), configPtr, configLength,
			&buf[0], int32(capacity), &outputLength, &outputOffset)
		if rc == CodeInsufficientCapacity {
			capacity = max(int(outputLength), capacity*2)
			continue
		}
		if err := Check(rc, "parse"); err != nil {
			return nil, 0, err
		}
		return buf[:outputLength], int(outputOffset), nil
	}
}

// StartSplit begins a streaming split. config must contain a split section with segment_id.
func StartSplit(edi []byte, mode ParseMode, config []byte) error {
	if err := ensure(); err != nil {
		return err
	}
	inputPtr, inputLength := buffer(edi)
	configPtr, configLength := buffer(config)
	return Check(cStartSplit(inputPtr, inputLength, int32(mode), configPtr, configLength), "start_split")
}

// Split advances the split stream and returns the size, offset, and whether this is the last step.
//
// Fetch the payload with GetResult(size) whenever size is greater than zero.
func Split() (size int, offset int, isLast bool, err error) {
	if err := ensure(); err != nil {
		return 0, 0, false, err
	}
	var resultSize, resultOffset int32
	var last uint8
	if err := Check(cSplit(&resultSize, &resultOffset, &last), "split"); err != nil {
		return 0, 0, false, err
	}
	return int(resultSize), int(resultOffset), last != 0, nil
}

// Build builds X12 EDI from transaction-set JSON.
//
// postfix is appended after each segment terminator, for example "\r\n".
// Pass "" for compact single-line output.
func Build(json []byte, postfix string) ([]byte, error) {
	if err := ensure(); err != nil {
		return nil, err
	}
	inputPtr, inputLength := buffer(json)
	var postfixPtr *byte
	if postfix != "" {
		postfixPtr = &append([]byte(postfix), 0)[0]
	}
	return withGrowth(max(4096, len(json)), "build", func(output []byte, outputLength *int32) int32 {
		return cBuild(inputPtr, inputLength, postfixPtr, &output[0], int32(len(output)), outputLength)
	})
}

// StartMerge begins a streaming merge from a full interchange JSON document.
func StartMerge(json []byte) error {
	if err := ensure(); err != nil {
		return err
	}
	ptr, length := buffer(json)
	return Check(cStartMerge(ptr, length), "start_merge")
}

// Merge returns the size of the next segment, or 0 at the end of the stream.
func Merge() (int, error) {
	if err := ensure(); err != nil {
		return 0, err
	}
	var size int32
	if err := Check(cMerge(&size), "merge"); err != nil {
		return 0, err
	}
	return int(size), nil
}

// GetResult copies the last split or merge result. size must match the reported size.
func GetResult(size int) ([]byte, error) {
	if size < 0 {
		return nil, errors.New("size must be zero or greater")
	}
	if err := ensure(); err != nil {
		return nil, err
	}
	result := make([]byte, size)
	ptr, _ := buffer(result)
	if err := Check(cGetResult(ptr, int32(size)), "get_result"); err != nil {
		return nil, err
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Convenience iterators
// ---------------------------------------------------------------------------

// SplitPart is one payload produced by IterSplit.
type SplitPart struct {
	Payload []byte
	Offset  int
	IsLast  bool
}

// IterSplit yields every split step that produces data. Iteration stops after the first error.
func IterSplit(edi []byte, mode ParseMode, config []byte) iter.Seq2[SplitPart, error] {
	return func(yield func(SplitPart, error) bool) {
		if err := StartSplit(edi, mode, config); err != nil {
			yield(SplitPart{}, err)
			return
		}
		for {
			size, offset, isLast, err := Split()
			if err != nil {
				yield(SplitPart{}, err)
				return
			}
			if size > 0 {
				payload, err := GetResult(size)
				if !yield(SplitPart{Payload: payload, Offset: offset, IsLast: isLast}, err) || err != nil {
					return
				}
			}
			if isLast {
				return
			}
		}
	}
}

// IterMerge yields each X12 segment produced by a merge stream. Iteration stops after the first error.
func IterMerge(json []byte) iter.Seq2[[]byte, error] {
	return func(yield func([]byte, error) bool) {
		if err := StartMerge(json); err != nil {
			yield(nil, err)
			return
		}
		for {
			size, err := Merge()
			if err != nil {
				yield(nil, err)
				return
			}
			if size == 0 {
				return
			}
			segment, err := GetResult(size)
			if !yield(segment, err) || err != nil {
				return
			}
		}
	}
}
