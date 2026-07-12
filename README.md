# Mullvad find fastest

Two Windows Go binaries for finding Mullvad city providers with good latency and throughput.

## Requirements

- Windows 10/11
- Go 1.26+ (`scoop install go`)
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

It runs both phases sequentially only when `-MaxPing` is provided; if omitted, it skips the ping phase and runs speed tests directly.

If you want to run manually, with custom options, you can run both programs sequentially. The benchmark phase starts only if the ping phase succeeds:

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

To skip the ping step explicitly, use `-SkipPing`:

```powershell
.\Start-MullvadBenchmark.ps1 -SkipPing -Exclude us -Exclude se-mma
```

For direct binary usage:

```powershell
& ".\mullvad-benchmark.exe" --skip-ping --output "benchmark.json"
```

Adjust `--max-ping 80` to the desired maximum latency in milliseconds. Remove or add `--exclude` options as needed.

For a short diagnostic run, use `--limit 1 --attempts 1` to probe only the first relay.

The ping binary temporarily adds only itself to Mullvad split tunneling, performs native Windows ICMP probes, writes the JSON atomically, and removes its exception. The benchmark binary then uses the VPN normally and tests eligible cities sequentially. Speed tests transfer several megabytes per city.

The benchmark now evaluates each city-provider bucket (for example wireguard
`000-099`, `100-199`, ...). `--exclude` can use `country`, `country-city`, or
`country-city-<provider-number>` to narrow results.

Example output (provider column added in current output):

```text
Rank  Country  City               Relays   Pre-ping  VPN latency  Download    Upload    Status
1     fr       Paris              3/10     83        53 ms        95.1        0.0       OK
2     be       Brussels           1/3      85        85 ms        93.6        0.0       OK
3     de       Berlin             1/8      74        70 ms        89.8        0.0       OK
4     fi       Helsinki           2/10     73        84 ms        86.9        0.0       OK
5     gb       London             2/15     183       75 ms        84.9        0.0       OK
6     ch       Zurich             2/11     101       76 ms        84.5        0.0       OK
7     at       Vienna             1/5      187       95 ms        83.9        0.0       OK
8     se       Stockholm          1/20     154       87 ms        76.2        0.0       OK
9     nl       Amsterdam          4/14     66        103 ms       75.9        0.0       OK
10    no       Stavanger          1/4      62        107 ms       75.7        0.0       OK
11    de       Frankfurt          3/18     66        84 ms        64.8        0.0       OK
12    gb       Manchester         1/6      63        95 ms        63.3        0.0       OK
13    no       Oslo               2/6      156       78 ms        62.8        0.0       OK
14    ro       Bucharest          1/2      106       125 ms       49.7        0.0       OK
15    ca       Montreal           4/14     69        143 ms       49.0        0.0       OK
16    us       Dallas, TX         1/17     64        197 ms       37.6        0.0       OK
17    us       Chicago, IL        1/19     163       156 ms       34.0        0.0       OK
18    ee       Tallinn            1/3      165       158 ms       30.6        0.0       OK
19    us       Seattle, WA        4/11     97        245 ms       27.9        0.0       OK
20    sg       Singapore          1/5      131       235 ms       26.1        0.0       OK
21    pe       Lima               1/2      141       237 ms       24.6        0.0       OK
22    ng       Lagos              1/2      46        189 ms       23.3        0.0       OK
23    th       Bangkok            1/2      81        256 ms       23.3        0.0       OK
24    ar       Buenos Aires       1/2      106       279 ms       22.0        0.0       OK
25    nz       Auckland           1/3      164       323 ms       17.8        0.0       OK
26    au       Sydney             1/11     169       303 ms       17.7        0.0       OK
27    de       Dusseldorf         1/3      59        120 ms       16.4        0.0       OK
28    us       Boston, MA         2/3      60        1383 ms      12.8        0.0       OK
29    jp       Osaka              1/4      156       358 ms       10.3        0.0       OK
30    br       Sao Paulo          1/5      54        382 ms       5.5         0.0       OK
```
