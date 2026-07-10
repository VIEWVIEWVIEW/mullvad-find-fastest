[CmdletBinding()]
param(
    [int]$MaxPing = 80,
    [string[]]$Exclude = @(),
    [string]$PingOutput = "mullvad-ping-results.json",
    [string]$BenchmarkOutput = "benchmark.json"
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$PingBinary = Join-Path $Root "mullvad-ping.exe"
$BenchmarkBinary = Join-Path $Root "mullvad-benchmark.exe"

if (-not (Test-Path -LiteralPath $PingBinary)) {
    throw "Missing $PingBinary. Build mullvad-ping.exe first."
}
if (-not (Test-Path -LiteralPath $BenchmarkBinary)) {
    throw "Missing $BenchmarkBinary. Build mullvad-benchmark.exe first."
}

Push-Location $Root
try {
    & $PingBinary --output $PingOutput
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }

    $BenchmarkArgs = @(
        "--input", $PingOutput,
        "--max-ping", $MaxPing,
        "--output", $BenchmarkOutput
    )
    foreach ($Location in $Exclude) {
        $BenchmarkArgs += @("--exclude", $Location)
    }

    & $BenchmarkBinary @BenchmarkArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
