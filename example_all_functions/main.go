// Walkthrough of every function in the ediFabric Native X12 C ABI.
//
// Each section calls one group of entry points and prints the result, so the
// output doubles as a reference for what each call returns.
//
// Usage (from the repository root):
//
//	go run ./example_all_functions
//	go run ./example_all_functions --serial YOUR_SERIAL
//	go run ./example_all_functions --lib /path/to/edifabric-x12-tools.dll
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ef "github.com/EdiFabric/edifabric-go-bindings"
)

// Free plan serial published in the ediFabric Native documentation.
const defaultSerial = "bd96a836feca45cb91c86ee65d281f52"

var (
	mapPath          string
	sampleEDI        []byte
	sampleEDIInvalid []byte
	parseConfig      []byte
	splitConfig      []byte
)

// Validation levels are 0 - no validation; 1 - syntax only; 2 - limits and codes; 3 - balancing; 4 - inter segment. The levels match the HIPAA SNIP validation levels.
var parseSettings = map[string]any{
	"validate": map[string]any{
		"regex":          nil,
		"date_format":    nil,
		"time_format":    nil,
		"skip_seq_count": false,
		"skip_hl_seq":    false,
		"snip_level":     4,
		"max_errors":     100,
	},
	"ack": map[string]any{
		"supress_ta1":   false,
		"ak901p":        false,
		"gen_for_valid": true,
		"gen997":        false,
	},
}

type modelMap struct {
	Default *string                   `json:"default"`
	Maps    map[string]map[string]any `json:"maps"`
}

func section(title string) {
	fmt.Println()
	fmt.Println(strings.Repeat("=", 70))
	fmt.Println(title)
	fmt.Println(strings.Repeat("=", 70))
}

func preview(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit] + " ..."
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func readMap() modelMap {
	var m modelMap
	check(json.Unmarshal(must(os.ReadFile(mapPath)), &m))
	return m
}

// loadLocalMap loads map/map.json and points each entry at the local map folder.
func loadLocalMap() modelMap {
	m := readMap()
	mapDir := must(filepath.Abs(filepath.Dir(mapPath)))
	for _, entry := range m.Maps {
		entry["location"] = mapDir
	}
	return m
}

// loadOnlineMap loads map/map.json and resolves unmapped transaction sets online with the serial.
func loadOnlineMap(serial string) modelMap {
	m := readMap()
	m.Default = &serial
	return m
}

func printMap(m modelMap) {
	keys := make([]string, 0, len(m.Maps))
	for key := range m.Maps {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	listed := strings.Join(keys, ", ")
	if listed == "" {
		listed = "(none)"
	}
	def := ""
	if m.Default != nil {
		def = *m.Default
	}
	fmt.Printf("  SetMap <- %s\n", mapPath)
	fmt.Printf("  default=%q, local maps=%s\n", def, listed)
}

// demoErrors calls GetError, RawGetError, FreeError.
func demoErrors() {
	section("Error messages: GetError, RawGetError, FreeError")

	for _, code := range []int{ef.CodeInsufficientCapacity, ef.CodeMapNotSet, ef.CodeLicenseNotSet} {
		fmt.Printf("  GetError(%3d) -> %s\n", code, ef.GetError(code))
	}

	// The raw export hands back a heap pointer that the caller owns.
	ptr := must(ef.RawGetError(ef.CodeTokenExpired))
	message := ef.DecodeString(ptr)
	ef.FreeError(ptr)
	fmt.Printf("  RawGetError(631) -> %s\n", message)
	fmt.Println("  FreeError(ptr) released the string")
}

// demoLogging calls InitLogger, GetAppVersion.
func demoLogging(logPath string) {
	section("Lifecycle: InitLogger, GetAppVersion")

	check(ef.InitLogger(logPath, ef.LogTrace))
	fmt.Printf("  InitLogger -> logging to %s\n", logPath)
	fmt.Printf("  GetAppVersion -> %d\n", must(ef.GetAppVersion()))
}

// demoTokenExpiration calls GetTokenExpiration.
func demoTokenExpiration() {
	section("Licensing: GetTokenExpiration")

	ticks := must(ef.GetTokenExpiration())
	when := "no token set"
	if ticks != 0 {
		when = ef.TicksToTime(ticks).String()
	}
	fmt.Printf("  GetTokenExpiration -> %d (%s)\n", ticks, when)
}

// demoSetOnlineMap calls SetMap.
func demoSetOnlineMap(serial string) {
	section("Model map: SetMap online, default is the serial key")

	m := loadOnlineMap(serial)
	check(ef.SetMap(must(json.Marshal(m))))
	printMap(m)
}

// demoSetLocalMap calls SetMap.
func demoSetLocalMap() {
	section("Model map: SetMap local, default is blank")

	m := loadLocalMap()
	check(ef.SetMap(must(json.Marshal(m))))
	printMap(m)
}

// demoParse calls Parse in mode 1.
func demoParse() []byte {
	section("Parse: Parse (mode 1, JSON only)")

	output, offset, err := ef.Parse(sampleEDI, ef.ParseJSON, nil)
	check(err)
	fmt.Printf("  %d bytes of JSON, offset=%d\n", len(output), offset)
	fmt.Printf("  %s\n", preview(string(output), 400))
	return output
}

// demoParseValidate calls Parse in mode 2.
func demoParseValidate() {
	section("Parse: Parse (mode 2, JSON + validation report)")

	output, offset, err := ef.Parse(sampleEDIInvalid, ef.ParseJSONValidate, parseConfig)
	check(err)
	fmt.Printf("  %d bytes total, validation starts at offset %d\n", len(output), offset)
	fmt.Printf("  validation -> %s\n", preview(string(output[offset:]), 400))
}

// demoParseAck calls Parse in mode 3.
func demoParseAck(title string, edi []byte) {
	section(title)

	output, offset, err := ef.Parse(edi, ef.ParseJSONValidateAck, parseConfig)
	check(err)
	fmt.Printf("  %d bytes total, report starts at offset %d\n", len(output), offset)
	fmt.Printf("  report -> %s\n", preview(string(output[offset:]), 600))
}

// demoSplit calls StartSplit, Split, GetResult.
func demoSplit() {
	section("Split: StartSplit, Split, GetResult")

	check(ef.StartSplit(sampleEDI, ef.ParseJSON, splitConfig))
	for step := 1; ; step++ {
		size, offset, isLast, err := ef.Split()
		check(err)
		var payload []byte
		if size > 0 {
			payload = must(ef.GetResult(size))
		}
		fmt.Printf("  step %d: size=%d offset=%d last=%t\n", step, size, offset, isLast)
		if len(payload) > 0 {
			fmt.Printf("    %s\n", preview(string(payload), 160))
		}
		if isLast {
			break
		}
	}
}

// demoBuild calls Build.
func demoBuild(parsedJSON []byte) {
	section("Build: Build")

	edi := string(must(ef.Build(parsedJSON, "\r\n")))
	fmt.Printf("  %d bytes of X12\n", len(edi))
	fmt.Println("  " + strings.TrimRight(strings.ReplaceAll(edi, "\r\n", "\n  "), " \n"))
}

// demoMerge calls StartMerge, Merge, GetResult.
func demoMerge(parsedJSON []byte) {
	section("Merge: StartMerge, Merge, GetResult")

	check(ef.StartMerge(parsedJSON))
	count := 0
	for {
		size := must(ef.Merge())
		if size == 0 {
			break
		}
		count++
		fmt.Printf("  segment %2d: %s\n", count, must(ef.GetResult(size)))
	}
	fmt.Printf("  Merge produced %d segments\n", count)
}

// demoIterators calls the IterSplit and IterMerge convenience wrappers.
func demoIterators(parsedJSON []byte) {
	section("Convenience wrappers: IterSplit, IterMerge")

	var sizes []int
	for part, err := range ef.IterSplit(sampleEDI, ef.ParseJSON, splitConfig) {
		check(err)
		sizes = append(sizes, len(part.Payload))
	}
	fmt.Printf("  IterSplit -> %d payloads, sizes %v\n", len(sizes), sizes)

	segments := 0
	for _, err := range ef.IterMerge(parsedJSON) {
		check(err)
		segments++
	}
	fmt.Printf("  IterMerge -> %d segments\n", segments)
}

// demoTeardown calls ClearCache, ShutdownLogger.
func demoTeardown() {
	section("Teardown: ClearCache, ShutdownLogger")

	check(ef.ClearCache())
	fmt.Println("  ClearCache -> map, license, stream state and results reset")
	check(ef.ShutdownLogger())
	fmt.Println("  ShutdownLogger -> logger stopped")
}

// runDemos runs every demo and converts an *ef.Error panic into an error.
func runDemos(serial string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var nativeErr *ef.Error
			if e, ok := r.(error); ok && errors.As(e, &nativeErr) {
				err = nativeErr
				return
			}
			panic(r)
		}
	}()

	// Community authorizes with the serial only. Developer can also use EnsureToken, Enterprise SetToken.
	check(ef.SetSerial(serial))
	fmt.Println("  SetSerial -> ok")

	demoTokenExpiration()

	demoSetLocalMap()

	parsedJSON := demoParse()

	demoSetOnlineMap(serial)

	demoParseValidate()
	demoParseAck("Parse: Parse (mode 3, JSON + validation + acknowledgment)", sampleEDI)
	demoParseAck("Parse: Parse (mode 3, invalid file, JSON + validation + acknowledgment)", sampleEDIInvalid)
	demoSplit()
	demoBuild(parsedJSON)
	demoMerge(parsedJSON)
	demoIterators(parsedJSON)
	demoTeardown()
	return nil
}

func run() int {
	serialFlag := flag.String("serial", "", "license serial (default: EDIFABRIC_SERIAL or the free plan serial)")
	libFlag := flag.String("lib", "", "path to edifabric-x12-tools.dll/.so/.dylib or its folder")
	rootFlag := flag.String("root", ".", "folder that contains the edi and map folders")
	flag.Parse()

	serial := *serialFlag
	if serial == "" {
		serial = os.Getenv("EDIFABRIC_SERIAL")
	}
	if serial == "" {
		serial = defaultSerial
	}

	mapPath = filepath.Join(*rootFlag, "map", "map.json")
	sampleEDI = must(os.ReadFile(filepath.Join(*rootFlag, "edi", "837p.txt")))
	sampleEDIInvalid = must(os.ReadFile(filepath.Join(*rootFlag, "edi", "837p_error.txt")))
	parseConfig = must(json.Marshal(parseSettings))
	splitSettings := map[string]any{"split": map[string]any{"segment_id": "LX", "segment_depth": 6, "loop_id": "2400"}}
	for key, value := range parseSettings {
		splitSettings[key] = value
	}
	splitConfig = must(json.Marshal(splitSettings))

	section("Load: LoadLibrary")
	library, err := ef.LoadLibrary(*libFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("  loaded %s\n", library.Path)

	demoErrors()

	demoLogging("edifabric.log")
	// Release the log file. On the success path demoTeardown already shut the logger down.
	defer ef.ShutdownLogger()

	if err := runDemos(serial); err != nil {
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		fmt.Fprintln(os.Stderr, "Check that the serial is valid and that map/map.json resolves the "+
			"transaction set to a local model file under map/.")
		return 1
	}

	section("Finished")
	fmt.Println("Every entry point in c-abi-edifabric_x12_tools.h was called.")
	return 0
}

func main() {
	os.Exit(run())
}
