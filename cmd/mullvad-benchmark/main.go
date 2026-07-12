package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/example/mullvad-benchmark/internal/bench"
)

func main() {
	input := flag.String("input", "mullvad-ping-results.json", "ping JSON path")
	output := flag.String("output", "", "optional benchmark JSON path")
	maxPing := flag.Float64("max-ping", 0, "only test city providers at or below this pre-ping in ms; zero disables")
	var excludes multiFlag
	flag.Var(&excludes, "exclude", "country code, country-city code, or provider filter; repeatable")
	connectTimeout := flag.Duration("connect-timeout", 45*time.Second, "connection timeout")
	flag.Parse()
	if *output == "" {
		*output = fmt.Sprintf("benchmark-results-%s.json", time.Now().Format("20060102-150405"))
	}
	b, err := os.ReadFile(*input)
	if err != nil {
		fatal(err)
	}
	var pf bench.PingFile
	if err := json.Unmarshal(b, &pf); err != nil {
		fatal(fmt.Errorf("invalid ping file: %w", err))
	}
	if pf.Version != 1 || len(pf.Relays) == 0 {
		fatal(fmt.Errorf("unsupported or empty ping file"))
	}
	if time.Since(pf.CreatedAt) > 24*time.Hour {
		fatal(fmt.Errorf("ping file is older than 24 hours"))
	}
	cities := bench.Cities(pf.Relays)
	selected := cities[:0]
	for _, c := range cities {
		if bench.Excluded(c, excludes) || (*maxPing > 0 && c.PrePingMS > *maxPing) {
			continue
		}
		selected = append(selected, c)
	}
	m := bench.Mullvad{Binary: "mullvad", Timeout: 20 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	original, _ := m.Status(ctx)
	wasConnected := strings.HasPrefix(strings.ToLower(strings.TrimSpace(original)), "connected")
	originalCountry, originalCity, hasOriginalLocation := bench.ParseLocation(original)
	defer restore(wasConnected, originalCountry, originalCity, hasOriginalLocation, m)
	results := make([]bench.CityResult, 0, len(selected))
	for _, city := range selected {
		city.Status = "FAILED"
		if err := m.SetLocation(ctx, city.CountryCode, city.CityCode, city.RelayName); err == nil {
			err = m.Connect(ctx)
		}
		if err == nil {
			err = bench.WaitConnected(ctx, m, *connectTimeout)
		}
		if err == nil {
			city.Speed, err = (bench.Cloudflare{Timeout: 45 * time.Second}).Test(ctx)
		}
		if err != nil {
			city.Error = err.Error()
		} else {
			city.Status = "OK"
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
		out := bench.BenchmarkFile{Version: 1, RunID: fmt.Sprintf("%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), InputRun: pf.RunID, Results: results}
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
	if c.Speed != nil {
		lat = fmt.Sprintf("%.0f ms", c.Speed.LatencyMS)
		down = fmt.Sprintf("%.1f", c.Speed.DownloadMB)
		up = fmt.Sprintf("%.1f", c.Speed.UploadMB)
	}
	fmt.Printf("%-5d %-8s %-18s %-9s %-10s %-9.0f %-12s %-11s %-9s %s\n", rank, c.CountryCode, c.City, provider, fmt.Sprintf("%d/%d", c.Reachable, c.RelayCount), c.PrePingMS, lat, down, up, c.Status)
}
func restore(connected bool, country, city string, hasLocation bool, m bench.Mullvad) {
	if !connected {
		_ = m.Disconnect(context.Background())
		return
	}
	if !hasLocation {
		fmt.Fprintln(os.Stderr, "warning: previous VPN relay could not be identified; leaving the current VPN connection active")
		return
	}
	ctx := context.Background()
	if err := m.SetLocation(ctx, country, city); err != nil {
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
