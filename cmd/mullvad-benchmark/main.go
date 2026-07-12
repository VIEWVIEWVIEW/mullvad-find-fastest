package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/example/mullvad-benchmark/internal/bench"
)

func main() {
	input := flag.String("input", "", "ping JSON path; optional when --skip-ping is set")
	skipPing := flag.Bool("skip-ping", false, "skip ping preselection and benchmark providers directly from relay list")
	output := flag.String("output", "", "optional benchmark JSON path")
	maxPing := flag.Float64("max-ping", 0, "only test city providers at or below this pre-ping in ms; zero disables")
	var excludes multiFlag
	flag.Var(&excludes, "exclude", "country code, country-city code, or provider filter; repeatable")
	connectTimeout := flag.Duration("connect-timeout", 45*time.Second, "connection timeout")
	flag.Parse()
	if *output == "" {
		*output = fmt.Sprintf("benchmark-results-%s.json", time.Now().Format("20060102-150405"))
	}
	if *input == "" {
		*skipPing = true
	}
	m := bench.Mullvad{Binary: "mullvad", Timeout: 20 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	inputRun := "direct"
	var cities []bench.CityResult
	if *skipPing {
		if *maxPing > 0 {
			fmt.Fprintln(os.Stderr, "warning: --max-ping is ignored when skipping ping")
		}
		relays, err := m.ListRelays(ctx)
		if err != nil {
			fatal(err)
		}
		cities = bench.CitiesFromRelays(relays)
	} else {
		var pf bench.PingFile
		b, readErr := os.ReadFile(*input)
		if readErr != nil {
			fatal(readErr)
		}
		if err := json.Unmarshal(b, &pf); err != nil {
			fatal(fmt.Errorf("invalid ping file: %w", err))
		}
		if pf.Version != 1 || len(pf.Relays) == 0 {
			fatal(fmt.Errorf("unsupported or empty ping file"))
		}
		if time.Since(pf.CreatedAt) > 24*time.Hour {
			fatal(fmt.Errorf("ping file is older than 24 hours"))
		}
		inputRun = pf.RunID
		cities = bench.Cities(pf.Relays)
	}
	selected := cities[:0]
	for _, c := range cities {
		if bench.Excluded(c, excludes) {
			continue
		}
		if !*skipPing && *maxPing > 0 && c.PrePingMS > *maxPing {
			continue
		}
		selected = append(selected, c)
	}

	if len(selected) == 0 {
		fatal(fmt.Errorf("no providers selected"))
	}

	original, _ := m.Status(ctx)
	wasConnected := strings.HasPrefix(strings.ToLower(strings.TrimSpace(original)), "connected")
	originalCountry, originalCity, originalRelay, hasOriginalLocation := bench.ParseRelayIdentity(original)
	wasMultihopEnabled, hasMultihopState := false, false
	if state, multihopErr := m.IsMultihopEnabled(ctx); multihopErr == nil {
		wasMultihopEnabled, hasMultihopState = state, true
		if state {
			logf("multihop enabled, disabling for benchmark")
			if err := m.SetMultihop(ctx, false); err != nil {
				fmt.Fprintln(os.Stderr, "warning: could not disable multihop:", err)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "warning: could not detect multihop state:", multihopErr)
	}
	defer restoreMultihop(hasMultihopState, wasMultihopEnabled, m)
	defer restore(wasConnected, originalCountry, originalCity, originalRelay, hasOriginalLocation, m)
	results := make([]bench.CityResult, 0, len(selected))
	for idx, city := range selected {
		logf("[%d/%d] testing %s/%s provider=%d relay=%s", idx+1, len(selected), city.CountryCode, city.CityCode, city.Provider, city.RelayName)
		city.Status = "FAILED"
		if ctx.Err() != nil {
			logf("[%d/%d] interrupted before test start; stopping", idx+1, len(selected))
			break
		}
		var err error
		if err = m.SetLocation(ctx, city.CountryCode, city.CityCode, city.RelayName); err == nil {
			logf("[%d/%d] connected location set, establishing tunnel", idx+1, len(selected))
			err = m.Connect(ctx)
		}
		if err == nil {
			err = bench.WaitConnected(ctx, m, *connectTimeout)
		}
		if err == nil {
			logf("[%d/%d] running speed test", idx+1, len(selected))
			city.Speed, err = (bench.Cloudflare{Timeout: 45 * time.Second}).Test(ctx)
		}
		if err != nil {
			city.Error = err.Error()
			logf("[%d/%d] failed: %s", idx+1, len(selected), err)
		} else {
			city.Status = "OK"
			logf("[%d/%d] complete: latency=%0.0fms dl=%0.1f mbps ul=%0.1f mbps", idx+1, len(selected), city.Speed.LatencyMS, city.Speed.DownloadMB, city.Speed.UploadMB)
		}
		if isInterrupted(ctx.Err()) {
			results = append(results, city)
			logf("[%d/%d] interrupted; stopping benchmark loop", idx+1, len(selected))
			break
		}
		results = append(results, city)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Speed == nil {
			return false
		}
		if results[j].Speed == nil {
			return true
		}
		return results[i].Speed.DownloadMB > results[j].Speed.DownloadMB
	})
	fmt.Printf("%-5s %-8s %-18s %-9s %-10s %-9s %-12s %-11s %-9s %s\n", "Rank", "Country", "City", "Provider", "Relays", "Pre-ping", "VPN latency", "Download", "Upload", "Status")
	for i, city := range results {
		printRow(i+1, city)
	}
	if *output != "" {
		out := bench.BenchmarkFile{Version: 1, RunID: fmt.Sprintf("%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), InputRun: inputRun, Results: results}
		if err := bench.WriteJSONAtomic(*output, out); err != nil {
			fatal(err)
		}
		writeCSV(*output+".csv", results)
	}
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, strings.ToLower(strings.TrimSpace(v)))
	return nil
}
func printRow(rank int, c bench.CityResult) {
	lat, down, up := "-", "-", "-"
	provider := "-"
	if c.Provider > 0 {
		provider = strconv.Itoa(c.Provider)
	}
	prePing := "-"
	if c.PrePingMS > 0 {
		prePing = fmt.Sprintf("%.0f", c.PrePingMS)
	}
	if c.Speed != nil {
		lat = fmt.Sprintf("%.0f ms", c.Speed.LatencyMS)
		down = fmt.Sprintf("%.1f", c.Speed.DownloadMB)
		up = fmt.Sprintf("%.1f", c.Speed.UploadMB)
	}
	fmt.Printf("%-5d %-8s %-18s %-9s %-10s %-9s %-12s %-11s %-9s %s\n", rank, c.CountryCode, c.City, provider, fmt.Sprintf("%d/%d", c.Reachable, c.RelayCount), prePing, lat, down, up, c.Status)
}
func logf(format string, args ...any) {
	prefix := time.Now().Format("15:04:05.000")
	fmt.Printf("[%s] %s\n", prefix, fmt.Sprintf(format, args...))
}
func restore(connected bool, country, city, relay string, hasLocation bool, m bench.Mullvad) {
	if !connected {
		_ = m.Disconnect(context.Background())
		return
	}
	if !hasLocation {
		fmt.Fprintln(os.Stderr, "warning: previous VPN relay could not be identified; leaving the current VPN connection active")
		return
	}
	ctx := context.Background()
	args := []string{country, city}
	if relay != "" {
		args = append(args, relay)
	}
	if err := m.SetLocation(ctx, args[0], args[1], args[2:]...); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not restore previous VPN location:", err)
		return
	}
	if err := m.Connect(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not reconnect previous VPN location:", err)
		return
	}
	if err := bench.WaitConnected(ctx, m, 45*time.Second); err != nil {
		fmt.Fprintln(os.Stderr, "warning: previous VPN location restoration timed out:", err)
	}
}
func isInterrupted(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
func restoreMultihop(enabled bool, hasState bool, m bench.Mullvad) {
	if !hasState || !enabled {
		return
	}
	if err := m.SetMultihop(context.Background(), true); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not restore multihop setting:", err)
	}
}
func writeCSV(path string, rows []bench.CityResult) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"country", "city", "provider", "relay", "relays", "pre_ping_ms", "latency_ms", "download_mbps", "upload_mbps", "status", "error"})
	for _, r := range rows {
		lat, down, up := "", "", ""
		if r.Speed != nil {
			lat = fmt.Sprintf("%.2f", r.Speed.LatencyMS)
			down = fmt.Sprintf("%.2f", r.Speed.DownloadMB)
			up = fmt.Sprintf("%.2f", r.Speed.UploadMB)
		}
		_ = w.Write([]string{r.CountryCode, r.City, fmt.Sprint(r.Provider), r.RelayName, fmt.Sprintf("%d/%d", r.Reachable, r.RelayCount), fmt.Sprintf("%.2f", r.PrePingMS), lat, down, up, r.Status, r.Error})
	}
	w.Flush()
}
func fatal(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }
