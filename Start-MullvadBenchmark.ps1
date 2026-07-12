[CmdletBinding()]
param(
    [int]$MaxPing = 80,
    [string[]]$Exclude = @(),
    [switch]$SkipPing,
    [string]$PingOutput = "",
    [string]$BenchmarkOutput = "benchmark.json"
)

$ErrorActionPreference = "Stop"
$Root = $PSScriptRoot
$PingBinary = Join-Path $Root "mullvad-ping.exe"
$BenchmarkBinary = Join-Path $Root "mullvad-benchmark.exe"

if (-not (Test-Path -LiteralPath $BenchmarkBinary)) {
    throw "Missing $BenchmarkBinary. Build mullvad-benchmark.exe first."
}
if (-not $SkipPing -and -not [string]::IsNullOrWhiteSpace($PingOutput) -and -not (Test-Path -LiteralPath $PingBinary)) {
    throw "Missing $PingBinary. Build mullvad-ping.exe first."
}

if ([string]::IsNullOrWhiteSpace($PingOutput)) {
    $SkipPing = $true
}

Push-Location $Root
try {
    if (-not $SkipPing) {
        & $PingBinary --output $PingOutput
        if ($LASTEXITCODE -ne 0) {
            exit $LASTEXITCODE
        }
    }

    $BenchmarkArgs = @(
        "--output", $BenchmarkOutput
    )
    if ($SkipPing) {
        $BenchmarkArgs += "--skip-ping"
    } else {
        $BenchmarkArgs += @(
            "--input", $PingOutput,
            "--max-ping", $MaxPing
        )
    }
    foreach ($Location in $Exclude) {
        $BenchmarkArgs += @("--exclude", $Location)
    }

    & $BenchmarkBinary @BenchmarkArgs
    exit $LASTEXITCODE
}
finally {
    Pop-Location
}
