# Mullvad benchmark

Two Windows Go binaries for finding Mullvad cities with good latency and throughput.

## Requirements

- Windows 10/11
- Go 1.26+
- Mullvad app installed, logged in, and its CLI available as `mullvad`
- ICMP allowed by the local firewall

## Build

```powershell
go build -o mullvad-ping.exe ./cmd/mullvad-ping
go build -o mullvad-benchmark.exe ./cmd/mullvad-benchmark
```

## Run

The recommended launcher is:

```powershell
.\Start-MullvadBenchmark.ps1 -MaxPing 80 -Exclude us,se-mma
```

It runs both phases sequentially and only starts the benchmark if the ping phase succeeds.

Run both phases sequentially. The benchmark phase starts only if the ping phase succeeds:

```powershell
& ".\mullvad-ping.exe" --output "mullvad-ping-results.json"

if ($LASTEXITCODE -eq 0) {
    & ".\mullvad-benchmark.exe" `
        --input "mullvad-ping-results.json" `
        --max-ping 80 `
        --exclude us `
        --exclude se-mma `
        --output "benchmark.json"
}
```

Adjust `--max-ping 80` to the desired maximum latency in milliseconds. Remove or add `--exclude` options as needed.

For a short diagnostic run, use `--limit 1 --attempts 1` to probe only the first relay.

The ping binary temporarily adds only itself to Mullvad split tunneling, performs native Windows ICMP probes, writes the JSON atomically, and removes its exception. The benchmark binary then uses the VPN normally and tests eligible cities sequentially. Speed tests transfer several megabytes per city.
