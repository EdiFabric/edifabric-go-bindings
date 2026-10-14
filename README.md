# ediFabric Native X12 - Go bindings

**ediFabric Native** is a self-contained, high-performance X12 EDI native shared library. It converts X12 EDI to JSON (and back),
validates transaction sets, and generates acknowledgments — callable from **any
language with a C foreign-function interface** (C, C++, Rust, Go, Python, Node.js,
Java/JNA, .NET, …). No .NET runtime, JVM, or other dependency is required on the
target machine.

Go bindings for [ediFabric Native](https://www.edifabric.com/edifabric-native.html), built on [purego](https://github.com/ebitengine/purego).
They load the library at runtime, so **cgo and a C compiler are not required**.

| File | Purpose |
| --- | --- |
| `edifabric_x12.go` | bindings for the complete C ABI (package `edifabricx12`) |
| `load_windows.go`, `load_unix.go`, `bundled.go` | platform library loading |
| `native/` | separate module that embeds the native library for each platform |
| `example_all_functions/main.go` | runnable walkthrough of every entry point |
| `c-abi-edifabric_x12_tools.h` | the C header these bindings mirror |
| `PUBLISHING.md` | how the modules are released |

## Install

```bash
go get github.com/EdiFabric/edifabric-go-bindings@v1.1.1
go get github.com/EdiFabric/edifabric-go-bindings/native@v1.1.1
```

The second module is optional. Import it once, anywhere in your program, to embed the
ediFabric Native library for the target platform: Windows x64, Linux x64 and arm64
(glibc 2.34 or later), or macOS arm64 (12 or later).

```go
import _ "github.com/EdiFabric/edifabric-go-bindings/native"
```

On first use the library is written to the user cache folder under `edifabric-x12/`
and loaded from there. A library you pass to `LoadLibrary`, set in
`EDIFABRIC_X12_LIB`, or put beside the executable or in the working directory takes
precedence over the embedded one. Leave the import out to ship the library yourself
and keep about 8 MB out of the binary.

## Requirements

- Go 1.25 or later
- 64-bit Windows, Linux, or macOS
- The native library for your platform (embedded by the `native` module; download it when you don't import that module):

| Platform | File |
| --- | --- |
| Windows | `edifabric-x12-tools.dll` |
| Linux | `edifabric-x12-tools.so` |
| macOS | `edifabric-x12-tools.dylib` |

1. [Sign up free for **Community**](https://www.edifabric.com/pricing.html) to get an evaluation serial key. Community never expires, requires no credit card, and is for non-production use. The first 14 days after you subscribe have unlimited operations, then the quota is 250 per day. To extend the unlimited period, ask through the [support portal](https://support.edifabric.com/hc/en-us/requests/new) or at [support@edifabric.com](mailto:support@edifabric.com). After signup, retrieve your serial from [Your Account](https://www.edifabric.com/docs/getting-started/your-account.html).
2. [Download the **ediFabric Native** library](https://www.edifabric.com/docs/edifabric-native/download.html).

Plus your **model files** (per transaction set) and a **map file** that tells the
engine where to find them. See [Model map](#model-map) for details.

## Getting started

**Sign up free for Community** at [edifabric.com/pricing](https://www.edifabric.com/pricing.html)
to get an evaluation serial key, then **download the library** from
[here](https://www.edifabric.com/docs/edifabric-native/download.html).
Put the native library in the repository root, then run the walkthrough from there with your serial:

```bash
go run ./example_all_functions --serial YOUR_SERIAL
```

It authorizes with your Community (or paid) serial, loads the model map, and
calls every function in the ABI, printing what each one returns.

```
======================================================================
Parse: Parse (mode 2, JSON + validation report)
======================================================================
  8711 bytes total, validation starts at offset 8556
  validation -> {"errors":[{"index":9,"element_index":2,"composite_index":1,"repetition_index":1,"code":17,"message":"Invalid EDI code"}],"errors_count":1,"data_count":46}
```

Options:

```bash
go run ./example_all_functions --serial YOUR_SERIAL   # Community or paid serial (required)
go run ./example_all_functions --lib /opt/edifabric   # library file or folder
go run ./example_all_functions --root /path/to/repo   # folder with the edi and map folders
```

You can also set `EDIFABRIC_SERIAL` instead of passing `--serial`.

The library path is resolved from `--lib`, then `EDIFABRIC_X12_LIB`, then the
executable's folder, the working directory, its parent, and finally the library
embedded by the `native` module, if imported. The serial comes from
`--serial`, then `EDIFABRIC_SERIAL`.

All strings and payloads cross the boundary as **UTF‑8 byte buffers**
(`pointer + length`). Every function returns `0` on success or a non-zero
[error code](#error-codes).

## Usage

Add the module to your project:

```bash
go get github.com/EdiFabric/edifabric-go-bindings
```

```go
package main

import (
	"fmt"
	"log"
	"os"

	ef "github.com/EdiFabric/edifabric-go-bindings"
)

func main() {
	serial := "your-serial" // from your Community or paid plan

	if _, err := ef.LoadLibrary(""); err != nil { // or ef.LoadLibrary(`C:\libs\edifabric-x12-tools.dll`)
		log.Fatal(err)
	}
	// Community: SetSerial. Developer: prefer EnsureToken. Enterprise: prefer SetToken.
	if err := ef.SetSerial(serial); err != nil {
		log.Fatal(err)
	}
	if err := ef.SetMap([]byte(`{"default": "` + serial + `", "maps": {}}`)); err != nil {
		log.Fatal(err)
	}

	edi, err := os.ReadFile("purchase-order.edi")
	if err != nil {
		log.Fatal(err)
	}
	transactions, _, err := ef.Parse(edi, ef.ParseJSON, nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(transactions))
}
```

Inputs and outputs are `[]byte`; serials, tokens, and paths are `string`.
Read a file from a subfolder of the working directory:

```go
edi, err := os.ReadFile(filepath.Join("edi", "837p.txt"))
```

### Validation and acknowledgments

`Parse` returns `output, offset, err`. In modes 2 and 3, `offset` marks where the
validation and acknowledgment JSON starts inside the output.

```go
config := []byte(`{
  "validate": {"snip_level": 2, "max_errors": 0},
  "ack": {"gen997": false, "supress_ta1": false}
}`)

output, offset, err := ef.Parse(edi, ef.ParseJSONValidateAck, config)
if err != nil {
	log.Fatal(err)
}
transactions := output[:offset]
var report struct {
	ErrorsCount int    `json:"errors_count"`
	Ack         string `json:"ack"`
}
if err := json.Unmarshal(output[offset:], &report); err != nil {
	log.Fatal(err)
}
fmt.Println(report.ErrorsCount, report.Ack)
```

| Mode | Constant | Output |
| --- | --- | --- |
| 1 | `ParseJSON` | transaction-set JSON |
| 2 | `ParseJSONValidate` | JSON plus a validation report |
| 3 | `ParseJSONValidateAck` | JSON plus validation and a 999/997/TA1 acknowledgment |

### Streaming large interchanges

`IterSplit` streams one transaction set (or repeating loop) at a time with flat
memory use. `segment_id` must be `ST` or the first segment of a repeating loop.

```go
config := []byte(`{"split": {"segment_id": "LX", "segment_depth": 6, "loop_id": "2400"}}`)

for part, err := range ef.IterSplit(edi, ef.ParseJSON, config) {
	if err != nil {
		log.Fatal(err)
	}
	handle(part.Payload)
}
```

`IterMerge` streams a full interchange JSON document back out one segment at a time:

```go
out, err := os.Create("out.edi")
if err != nil {
	log.Fatal(err)
}
defer out.Close()
for segment, err := range ef.IterMerge(transactions) {
	if err != nil {
		log.Fatal(err)
	}
	out.Write(segment)
	out.WriteString("\r\n")
}
```

Both wrap the underlying `StartSplit`/`Split`/`GetResult` and
`StartMerge`/`Merge`/`GetResult` sequences, which you can also drive directly.
Iteration stops after the first error.

### Building EDI

```go
ediText, err := ef.Build(transactions, "\r\n") // "" for compact output
```

## API reference

Conventions used by every function:

- Returns `int`: `0` = success, non-zero = [error code](#error-codes).
- Inputs are UTF‑8 `byte*` + `int length`.
- Output functions use **grow-and-retry**: if the buffer is too small the call
  returns `1` (`InsufficientCapacity`) and writes the required size into the
  length out-parameter; reallocate and call again.
- Exceptions never cross the boundary.

Every wrapper returns an `*ef.Error` (with `Code`, `Message`, and `API`, the C
function name) when the native call returns a non-zero status. Use `errors.As` to
inspect it. Buffer growth (`InsufficientCapacity`) is retried automatically. Any
wrapper loads the library on first use if you haven't called `LoadLibrary`.

```go
var nativeErr *ef.Error
if errors.As(err, &nativeErr) && nativeErr.Code == ef.CodeLicenseQuotaExceeded {
	// Community daily quota reached
}
```

| Group | Functions |
| --- | --- |
| Loading | `LoadLibrary`, `LibraryFilename` |
| Lifecycle | `InitLogger`, `ShutdownLogger`, `ClearCache` |
| Licensing | `EnsureToken`, `GetAppVersion`, `GetToken`, `ValidateToken`, `SetToken`, `GetTokenExpiration`, `TicksToTime`, `SetSerial` |
| Model map | `SetMap` |
| Processing | `Parse`, `StartSplit`, `Split`, `Build`, `StartMerge`, `Merge`, `GetResult` |
| Errors | `GetError`, `RawGetError`, `FreeError`, `DecodeString`, `Check` |
| Wrappers | `IterSplit`, `IterMerge` |
| Constants and types | `ParseMode`, `LogLevel`, `Code…` error codes, `Error`, `SplitPart`, `Library` |

`GetError` frees the native string for you. Call `FreeError` only if you
invoke the raw export yourself through `RawGetError` (read the pointer with
`DecodeString`). Builds that do not export `free_error` fall back to the C runtime
`free`.

## Licensing

> [!NOTE]
> Sign up free for the [Community plan](https://www.edifabric.com/pricing.html)
> to get an evaluation serial key. Community never expires, requires no credit
> card, and is for non-production evaluation, learning, and prototyping
> (unlimited operations for the first 14 days after you subscribe, then 250 per day;
> to extend the unlimited period, ask through the [support portal](https://support.edifabric.com/hc/en-us/requests/new)
> or at [support@edifabric.com](mailto:support@edifabric.com)). After signup, copy your serial from
> [Your Account](https://www.edifabric.com/docs/getting-started/your-account.html).
>
> If you hit the Community daily quota, native calls return [error 639](#error-codes);
> upgrade at [edifabric.com/pricing](https://www.edifabric.com/pricing.html) if you
> want to continue.

| Plan | What works | Recommended |
| --- | --- | --- |
| Community | `SetSerial` only | `SetSerial` |
| Developer | `SetSerial` and `EnsureToken` (`EnsureToken` caches the result for 1 day) | `EnsureToken` |
| Enterprise | `SetSerial`, `EnsureToken`, `GetToken` / `SetToken` | `SetToken` (offline tokens) |

```go
// Community: authorize per process against the license server
err := ef.SetSerial(serial)

// Developer (recommended): 1-day built-in cache; refreshes if the token expires within N seconds
err = ef.EnsureToken(serial, 3600)
ticks, err := ef.GetTokenExpiration() // .NET UTC ticks, 0 when unset
fmt.Println(ef.TicksToTime(ticks))

// Developer (also works): same as Community, online check per process
err = ef.SetSerial(serial)

// Enterprise (recommended): fetch / validate / set an offline token yourself
token, err := ef.GetToken(serial)
err = ef.ValidateToken(token)
err = ef.SetToken(token)
```

## Model map

`SetMap` tells the engine where to find transaction-set models. Keys are
`message:version`. Set `default` to your serial to resolve unmapped transaction
sets through the online spec service, or leave it `null` (or `""`) and map
everything locally.

The example builds that JSON at runtime instead of hard-coding paths. Online
fallback is a map with `default` set to your serial:

```go
mapJSON, _ := json.Marshal(map[string]any{"default": serial, "maps": map[string]any{}})
err := ef.SetMap(mapJSON)
```

For local models, load a map file and rewrite each entry's `location` to the
folder that actually holds the JSON files (see `loadLocalMap` in
`example_all_functions/main.go`):

```go
mapPath := filepath.Join("map", "map.json")
mapDir, _ := filepath.Abs(filepath.Dir(mapPath))

var localMap struct {
	Default string                    `json:"default"`
	Maps    map[string]map[string]any `json:"maps"`
}
data, _ := os.ReadFile(mapPath)
json.Unmarshal(data, &localMap)
for _, entry := range localMap.Maps {
	entry["location"] = mapDir
}

mapJSON, _ := json.Marshal(localMap)
err := ef.SetMap(mapJSON)
```

`map.json` lists each transaction set; `location` is filled in at runtime so the
same file works from any working directory:

```json
{
  "default": "",
  "maps": {
    "837:005010X222A1": { "type": 1, "name": "model837P.json", "location": "" },
    "834:005010X220A1": { "type": 1, "name": "model834.json",  "location": "" }
  }
}
```

You can also mix both: keep `default` as your serial and add local entries under
`maps` for the transaction sets you ship on disk.

All X12 transactions, such as 837P, 834 and 850, are represented as ediFabric Native JSON models. The models are the same on every plan, Community or paid.

**Standard models** come from the [EDI spec library](https://www.edifabric.com/specs/index.html), and you can download them without an account. Open the transaction, for example [837P](https://www.edifabric.com/specs/x12/hipaa/005010/837p.html), and on the **ediFabric Native** tab select **Download Native**.

![The Download Native button on the ediFabric Native tab of 837P in the EDI spec library](model.png)

**Custom models** come from the [EDI Spec Builder](https://www.edifabric.com/spec-builder/index.html). When a trading partner changes the standard, open the transaction in the EDI spec library and select **Customize in Spec Builder**, or import your own OpenEDI file. Edit the OpenEDI schema until it matches the partner's guide and select **Update**. Then, on the **ediFabric Native** tab, select **Download Native**.

![The Download Native button on the ediFabric Native tab of a custom spec in the EDI Spec Builder](model-builder.png)

Save the downloaded file where the map can name it, and add it to `maps` under its `message:version` key, for example `837:005010X222A1`. The file is the compiled layout the engine loads, not the OpenEDI schema. To change a model, edit the spec in the EDI Spec Builder and download it again. See [EDI models](https://www.edifabric.com/docs/edifabric-native/edi-models.html) and [OpenEDI format](https://www.edifabric.com/docs/getting-started/openedi-format.html).

## Configuration JSON

All JSON uses **`snake_case`** keys and is case-insensitive.

### ParseConfig (`Parse`)

```json
{
  "validate": { "regex": null, "date_format": null, "time_format": null,
                "skip_seq_count": false, "skip_hl_seq": false,
                "snip_level": 0, "max_errors": 0 },
  "ack":      { "supress_ta1": false, "ak901p": false,
                "gen_for_valid": false, "gen997": false }
}
```

- `validate` — applied when `mode ≥ 2`; `snip_level` is `1`–`4`.
- `ack` — applied when `mode == 3`.

All sections are optional for `Parse`.

### SplitConfig (`StartSplit`)

```json
{
  "split":    { "segment_id": "ST", "segment_depth": 0, "loop_id": null }
}
```

- `split` — required for `StartSplit`.

Splitting is possible for the following boundaries:

- Transaction - for files that contain batches of transactions.
- Repeating loop - for files that contain batches of loops, such as order lines, claims or benefit enrollments.

The splitter must be configured as follows:

- `segment_id`  — the name of the segment to split by. It must be either ST or the first segment in the repeatable loop (Mandatory).
- `segment_depth` — the depth of the segment in the model hierarchy (Mandatory).
- `loop_id` — the name of the loop for the segment specified in segment_id (Optional).

The values for the splitter are in [EdiNation](https://edination.edifabric.com/), the free X12 editor. Load a file, select **Parse**, and select the first segment of the loop in the **Structure** tab. For example, to split an 837P by loop 2000A, load the 837P sample from **Samples** and select the **HL** segment in loop 2000A. Under **Selected item**, `segment_id` is **Code**, `loop_id` is the last item in **Path**, and `segment_depth` is **Depth**.

> [!NOTE]
> If a segment shows no **Splitter**, the file can't be split by that segment.

The easiest way to get the splitter configuration is to select the copy button next to **Splitter**, which copies the three values as JSON. Paste them as the value of `split`.

![The Splitter of the HL segment in loop 2000A of an 837P, under Selected item in EdiNation](splitter.png)

## Threading

The library holds process-global state (model map, active split reader, active
merge writer, last result, license) behind an internal lock. `Parse` and `Build`
are independent per call and safe to run from several goroutines, but each split
or merge sequence must run to completion without another split or merge
interleaving from a different goroutine.

## Error codes

`0` is success and `1` means the output buffer was too small. Library-level codes
are exposed as `Code…` constants, and `GetError(code)` returns the message.
Validation codes (elements, segments, transaction sets, groups, and interchanges)
appear in the parse report when `mode ≥ 2`.

**Error 639** means the Community (evaluation) daily quota was exceeded.
Upgrade your plan at [edifabric.com/pricing](https://www.edifabric.com/pricing.html)
if you wish to continue.

### Parser and library

| Code | Meaning |
| --- | --- |
| 1 | The suggested output buffer size is too small |
| 501 | Unexpected error. Contact support@edifabric.com and include a sample project/file to reproduce the issue |
| 502 | No connection to EdiNation API |
| 503 | The model map configuration is invalid. Check the paths and the model file names are correct |
| 611 | The input buffer is either null or its size is nill |
| 612 | The logger failed to log |
| 613 | The map configuration file is invalid |
| 614 | The output capacity must be positive |
| 615 | Models map must be set before parsing or splitting |
| 616 | Mode must be any of: 1 - Parse, 2 - Parse and Validate, 3 - Parse and Validate and Acknowledge |
| 617 | Parser failed. Contact support@edifabric.com and include a sample project/file to reproduce the issue |
| 618 | Validation failed. Contact support@edifabric.com and include a sample project/file to reproduce the issue |
| 619 | Validation serializer failed. Contact support@edifabric.com and include a sample project/file to reproduce the issue |
| 620 | The token is invalid. Contact support@edifabric.com for assistance |
| 621 | The configuration file is invalid |
| 622 | The split segment ID must not be blank |
| 623 | Call `StartSplit` before splitting |
| 624 | The result can't be retrieved. Contact support@edifabric.com and include a sample project/file to reproduce the issue |
| 625 | Result buffer size mismatched |
| 626 | Call `StartMerge` before merging |
| 627 | The output buffer is either null or its size is nill |
| 628 | The serial number is missing or incorrect. `GetToken` doesn't work with Developer license |
| 629 | License was not installed. Contact support@edifabric.com for assistance |
| 630 | No license to use this version. Contact support@edifabric.com for assistance |
| 631 | The token has expired. Get and set a new token to continue |
| 632 | The token is missing. Set token to continue |
| 633 | Reached the maximum number of licenses. Set token to continue |
| 634 | Environment not recognized for licensing or reached the maximum number of licenses |
| 635 | Serial or token not found. Either set token or serial to continue |
| 636 | The rate to get serials was exceeded for your license. Wait for 60 seconds and try again or upgrade your license |
| 637 | Invalid JSON. Enable logging for additional details |
| 638 | The operation is not supported by your license |
| 639 | Community daily quota exceeded. Your license has reached its daily call limit. Upgrade your plan at edifabric.com to continue |

## Troubleshooting

**`could not load edifabric-x12-tools.dll`** — the library is
not on any searched path. Pass `ef.LoadLibrary("/path/to/library")` or set
`EDIFABRIC_X12_LIB`. If the message ends with `cannot find function ...`, the
library is older than these bindings; download the current build.

**`open edi\837p.txt: The system cannot find the path specified`** — run the
example from the repository root, or pass `--root`.

**Error 615 on Parse** — call `SetMap` before parsing or splitting. `ClearCache`
resets the map, so reload it afterwards.

**Error 628 / 635 on Parse** — authorize first: `SetSerial` on Community,
`EnsureToken` (or `SetSerial`) on Developer, or `SetToken` on Enterprise.

**Error 633 on EnsureToken / GetToken** — the plan's machine quota is used up.
Contact support.

**Error 639** — the Community (evaluation) daily quota was exceeded. Wait until
the next day, or [upgrade your plan](https://www.edifabric.com/pricing.html) if
you wish to continue.

## Links

- [Documentation](https://www.edifabric.com/docs/edifabric-native/introduction.html)
- [Product page](https://www.edifabric.com/edifabric-native.html)
- [Community plan (free signup)](https://www.edifabric.com/pricing.html)
- [Your Account](https://www.edifabric.com/docs/getting-started/your-account.html)
- Support: support@edifabric.com
