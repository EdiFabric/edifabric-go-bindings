<#
.SYNOPSIS
Commit the four libraries into the native module, smoke-test it, and tag and push both modules.

.DESCRIPTION
Go modules are served from git tags, so a release is a commit with the libraries in native/bin/
plus the tags v<Version> (the bindings) and native/v<Version> (the libraries). Needs Go 1.25+ and
push rights to the remote. -DryRun stages and smoke-tests without committing, tagging, or pushing.

.EXAMPLE
./scripts/release.ps1 -Version 1.1.1 -DryRun
./scripts/release.ps1 -Version 1.1.1
#>
param(
    [Parameter(Mandatory)][ValidatePattern('^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?$')][string]$Version,
    [switch]$DryRun,
    [switch]$SkipFetch,
    [string]$Remote = 'origin'
)
$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$module = 'github.com/EdiFabric/edifabric-go-bindings'
$tags = "v$Version", "native/v$Version"

function Invoke-Checked {
    # Native tools write progress to stderr; only the exit code decides failure.
    $ErrorActionPreference = 'Continue'
    & $args[0] $args[1..($args.Count - 1)] 2>&1 | ForEach-Object { "$_" }
    if ($LASTEXITCODE -ne 0) { throw "$($args -join ' ') failed with exit code $LASTEXITCODE" }
}

if (-not $DryRun) {
    if (git status --porcelain) { throw 'Commit or stash your changes first; the release tags HEAD.' }
    foreach ($tag in $tags) {
        if (git tag --list $tag) { throw "Tag $tag already exists." }
    }
}

if (-not $SkipFetch) { & "$PSScriptRoot/fetch-natives.ps1" -Destination natives }
foreach ($rid in 'win-x64', 'linux-x64', 'linux-arm64', 'osx-arm64') {
    $library = Get-ChildItem "natives/$rid" -File | Where-Object Extension -ne '.asc'
    if (-not $library) { throw "natives/$rid has no library; run scripts/fetch-natives.ps1" }
    New-Item -ItemType Directory "native/bin/$rid" -Force | Out-Null
    Copy-Item $library.FullName "native/bin/$rid/"
}

foreach ($dir in 'native', 'scripts/smoke-test') {
    Push-Location $dir
    try {
        Invoke-Checked go mod edit "-require=$module@v$Version"
        if ($dir -eq 'scripts/smoke-test') { Invoke-Checked go mod edit "-require=$module/native@v$Version" }
        Invoke-Checked go mod tidy
    }
    finally { Pop-Location }
}

Invoke-Checked go vet ./...
Push-Location native
try {
    foreach ($target in 'windows/amd64', 'linux/amd64', 'linux/arm64', 'darwin/arm64', 'linux/riscv64') {
        $env:GOOS, $env:GOARCH = $target -split '/'
        Invoke-Checked go vet ./...
    }
}
finally {
    Remove-Item Env:GOOS, Env:GOARCH -ErrorAction SilentlyContinue
    Pop-Location
}

$smoke = Join-Path ([IO.Path]::GetTempPath()) ([IO.Path]::GetRandomFileName())
New-Item -ItemType Directory $smoke | Out-Null
try {
    $exe = Join-Path $smoke 'smoke-test.exe'
    Invoke-Checked go build -C scripts/smoke-test -o $exe .
    Push-Location $smoke
    try { Invoke-Checked $exe }
    finally { Pop-Location }
}
finally {
    Remove-Item $smoke -Recurse -Force -ErrorAction SilentlyContinue
}

if ($DryRun) {
    Write-Host "Dry run: staged the libraries in native/bin and smoke-tested; nothing committed, tagged, or pushed."
    return
}

Invoke-Checked git add native scripts/smoke-test
if (git status --porcelain) { Invoke-Checked git commit -m "Release v$Version" }
foreach ($tag in $tags) { Invoke-Checked git tag -a $tag -m "Release $tag" }
Invoke-Checked git push $Remote HEAD @tags

# Ask the module proxy to fetch both versions so `go get` sees them right away.
$env:GOPROXY = 'https://proxy.golang.org'
Push-Location ([IO.Path]::GetTempPath())
try {
    Invoke-Checked go mod download -json "$module@v$Version" "$module/native@v$Version"
}
finally {
    Remove-Item Env:GOPROXY
    Pop-Location
}
Write-Host "Released $module v$Version and $module/native v$Version."
