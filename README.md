# Mullvad find fastest

Tools to benchmark Mullvad relays, show results by city + provider bucket, and build Mullvad custom lists from selected results.

## Quick Start

```powershell
# 1) Build everything
go build -o mullvad-ping.exe ./cmd/mullvad-ping
go build -o mullvad-benchmark.exe ./cmd/mullvad-benchmark
go build -o mullvad-list-builder.exe ./cmd/mullvad-list-builder

# 2) Run recommended pipeline (ping first, then benchmark)
# Start-MullvadBenchmark.ps1 defaults -BenchmarkOutput to benchmark.json.
./Start-MullvadBenchmark.ps1 -MaxPing 80 -Exclude @("us","se-mma")

# 3) Build a custom list in the Mullvad client from best results
./mullvad-list-builder.exe --input benchmark.json --name "Fast relays"
```

To add the selected rows to an actual Mullvad custom list in the client, run:

```powershell
./mullvad-list-builder.exe --input benchmark.json --name "Fast relays"
```

## Requirements

- Windows 10/11
- Go 1.26+ (for building binaries)
- Mullvad app installed and logged in
- `mullvad` CLI available on PATH

## What each component does

- `Start-MullvadBenchmark.ps1` - orchestrates the workflow (recommended).
- `mullvad-ping.exe` - collects ICMP latency per relay.
- `mullvad-benchmark.exe` - runs download and upload speed tests.
- `mullvad-list-builder.exe` - interactively builds and applies a Mullvad custom list from city + provider buckets.

## Build all binaries

Run from repository root:

```powershell
go build -o mullvad-ping.exe ./cmd/mullvad-ping
go build -o mullvad-benchmark.exe ./cmd/mullvad-benchmark
go build -o mullvad-list-builder.exe ./cmd/mullvad-list-builder
```

## Prebuilt binaries

If you don’t want to build locally, download prebuilt executables from the
[GitHub Releases page](https://github.com/VIEWVIEWVIEW/mullvad-find-fastest/releases).

## Run with Start-MullvadBenchmark.ps1 (recommended)

`Start-MullvadBenchmark.ps1` does one of two things:

- ping + benchmark when `-MaxPing` is provided, or
- benchmark-only when no `-MaxPing` is provided.

### Parameters

| Parameter | Example | Description |
| --- | --- | --- |
| `-MaxPing <ms>` | `-MaxPing 80` | Ping threshold in ms. If provided, the script runs ping first and then benchmarks only entries at or below this ping value. Omit to skip ping entirely (unless `-SkipPing` is used). |
| `-Exclude <string[]>` | `-Exclude @("us","se-mma")` | Exclude one or more location filters. The parameter is an array; pass one array value instead of repeating the parameter. |
| `-SkipPing` | `-SkipPing` | Skip pinging and benchmark directly from current relay list. Equivalent to benchmark-only mode. |
| `-PingOutput <path>` | `-PingOutput c:\tmp\ping.json` | Optional custom output path for ping results. Defaults to `mullvad-ping-results.json`. |
| `-BenchmarkOutput <path>` | `-BenchmarkOutput c:\tmp\bench.json` | Optional custom output path for benchmark results. Defaults to `benchmark.json` in this wrapper script. |

### Examples

Run ping first and then benchmark only relays with pre-ping <= 80ms:

```powershell
./Start-MullvadBenchmark.ps1 -MaxPing 80 -Exclude @("us","se-mma") -BenchmarkOutput benchmark.json
```

Run benchmark only (skip ping):

```powershell
./Start-MullvadBenchmark.ps1 -SkipPing -Exclude @("fr","de-par-1")
```

Use explicit output files:

```powershell
./Start-MullvadBenchmark.ps1 -MaxPing 70 -PingOutput c:\tmp\ping.json -BenchmarkOutput c:\tmp\bench.json
```

## Run binaries directly

Use direct binaries when you need full control.

### `mullvad-ping.exe`

#### Flags

- `--output <path>` (default: `mullvad-ping-results.json`)
- `--attempts <int>` (default: `3`)
- `--timeout-ms <int>` (default: `1500`)
- `--limit <int>` (default: `0` = all relays)

Example:

```powershell
./mullvad-ping.exe --output "mullvad-ping-results.json" --attempts 3 --timeout-ms 1500 --limit 0
```

### `mullvad-benchmark.exe`

#### Flags

- `--input <path>`: ping JSON input path.
  - Required if `--skip-ping` is not set.
- `--skip-ping`: skip ping input and benchmark all relays from current relay list.
- `--output <path>`: benchmark JSON output (default: timestamped `benchmark-results-YYYYMMDD-HHMMSS.json`).
- `--max-ping <ms>`: pre-filter relays by ping latency when ping input is used. `0` disables pre-filtering.
- `--exclude <string>` (repeatable): filter by `country`, `country-city`, or `country-city-<provider-number>`.
  - Examples: `us`, `de-par`, `de-par-2`, `wg-001`
- `--connect-timeout <duration>` (default: `45s`)

From ping results:

```powershell
./mullvad-benchmark.exe --input "mullvad-ping-results.json" --max-ping 80 --exclude us --exclude se-mma --output "benchmark.json"
```

Benchmark only:

```powershell
./mullvad-benchmark.exe --skip-ping --connect-timeout 45s --output "benchmark.json"
```

Output files:

- `benchmark.json` (JSON)
- `benchmark.json.csv` (CSV)

### `mullvad-list-builder.exe`

You can build and apply a custom list directly inside the Mullvad client from a benchmark file.

#### Flags

- `--input <path>`: benchmark JSON path (default: `benchmark.json` or latest `benchmark-*.json`).
- `--name <string>`: target custom list name.
- `--include-failed`: include entries that failed speed tests.
- `--append`: append to existing list.
- `--limit <int>`: limit rows shown in selector.
- `--timeout <duration>`: CLI timeout for Mullvad commands (default: `30s`).

Example:

```powershell
./mullvad-list-builder.exe --input "benchmark.json" --name "Fast Paris/Frankfurt" --append --limit 50 --timeout 30s
```

If `--name` is omitted, you are prompted for it.

### List-builder controls

- ↑ / ↓: move selection cursor
- Space: toggle current row
- Enter: add selected city+provider buckets
- Q: quit without changes

Each row represents a city+provider bucket (for example `000-099`, `100-199`) and adds all relay hostnames for that bucket into the chosen Mullvad custom list.

## Notes

- `Start-MullvadBenchmark.ps1` needs `mullvad-benchmark.exe` always; it only needs `mullvad-ping.exe` when `-MaxPing` is used.
- You can run `mullvad-benchmark.exe` with `--input` and `--max-ping 0` to keep all ping results.

