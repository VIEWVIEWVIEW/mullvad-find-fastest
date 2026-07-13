package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/example/mullvad-benchmark/internal/bench"
)

type selectionRow struct {
	countryCode string
	cityCode    string
	countryName string
	cityName    string
	provider    int

	providerRange  string
	providerHost   string
	providerStatus string
	providerSpeed  string

	prePingMS  float64
	latencyMS  float64
	downloadMB float64
	uploadMB   float64
	relays     []string
	relayCount int
}

func main() {
	input := flag.String("input", "", "benchmark JSON path (defaults to benchmark.json or latest benchmark-*.json)")
	listName := flag.String("name", "", "name of the Mullvad custom list")
	includeFailed := flag.Bool("include-failed", false, "include benchmark rows without speed results")
	appendList := flag.Bool("append", false, "append to an existing list instead of replacing it")
	limit := flag.Int("limit", 0, "only keep the top N rows before selection")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for Mullvad CLI calls")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	path, err := resolveBenchmarkInput(*input)
	if err != nil {
		fatal(err)
	}
	benchFile, err := loadBenchmarkFile(path)
	if err != nil {
		fatal(err)
	}
	if len(benchFile.Results) == 0 {
		fatal(fmt.Errorf("benchmark file has no results: %s", path))
	}

	m := bench.Mullvad{Binary: "mullvad", Timeout: *timeout}
	relays, err := m.ListRelays(ctx)
	if err != nil {
		fatal(err)
	}

	rows := buildRows(benchFile, relays, *includeFailed)
	if len(rows) == 0 {
		fatal(fmt.Errorf("no eligible provider buckets found"))
	}
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
	}

	if *listName == "" {
		*listName, err = promptListName(m, ctx)
		if err != nil {
			fatal(err)
		}
	}
	if strings.TrimSpace(*listName) == "" {
		fatal(fmt.Errorf("custom list name cannot be empty"))
	}

	selectedIdx, err := promptForSelection(rows)
	if err != nil {
		if errors.Is(err, errSelectionCancelled) {
			fmt.Println("selection cancelled")
			return
		}
		fatal(err)
	}
	if len(selectedIdx) == 0 {
		fatal(fmt.Errorf("no providers selected"))
	}

	if err := provisionCustomList(ctx, m, *listName, *appendList); err != nil {
		fatal(err)
	}

	added, failed, err := applySelection(ctx, m, *listName, rows, selectedIdx)
	if err != nil {
		fatal(err)
	}
	fmt.Printf("Added %d relays from %d provider buckets to %q\n", added, len(selectedIdx), *listName)
	if failed > 0 {
		fmt.Printf("Failed to add %d relays. See mullvad CLI errors above.\n", failed)
	}
}

func resolveBenchmarkInput(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("input file does not exist: %s", path)
	}
	if _, err := os.Stat("benchmark.json"); err == nil {
		return "benchmark.json", nil
	}
	candidates, err := filepath.Glob("benchmark*.json")
	if err != nil {
		return "", err
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("no benchmark JSON file found")
	}
	best := candidates[0]
	bestTime := time.Time{}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.ModTime().After(bestTime) {
			best = candidate
			bestTime = info.ModTime()
		}
	}
	return best, nil
}

func loadBenchmarkFile(path string) (bench.BenchmarkFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return bench.BenchmarkFile{}, err
	}
	var payload bench.BenchmarkFile
	if err := json.Unmarshal(b, &payload); err != nil {
		return bench.BenchmarkFile{}, err
	}
	return payload, nil
}

func buildRows(file bench.BenchmarkFile, relays []bench.Relay, includeFailed bool) []selectionRow {
	relayBuckets := make(map[bucketKey][]string)
	for _, relay := range relays {
		key := bucketKey{
			country:  relay.CountryCode,
			city:     relay.CityCode,
			provider: relay.Provider,
		}
		relayBuckets[key] = append(relayBuckets[key], relay.Name)
	}
	for key := range relayBuckets {
		uniqueSorted(relayBuckets[key])
	}

	seen := map[bucketKey]struct{}{}
	rows := make([]selectionRow, 0, len(file.Results))
	for _, result := range file.Results {
		if result.Provider <= 0 {
			continue
		}
		if !includeFailed && result.Speed == nil {
			continue
		}
		key := bucketKey{
			country:  result.CountryCode,
			city:     result.CityCode,
			provider: result.Provider,
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}

		names := append([]string{}, relayBuckets[key]...)
		if len(names) == 0 && result.RelayName != "" {
			names = append(names, result.RelayName)
		}
		if len(names) == 0 {
			continue
		}

		row := selectionRow{
			countryCode:   result.CountryCode,
			cityCode:      result.CityCode,
			countryName:   result.Country,
			cityName:      result.City,
			provider:      result.Provider,
			providerRange: result.ProviderRange,
			providerHost:  nonEmpty(result.ProviderHost, "-"),
			relayCount:    len(names),
			relays:        names,
		}
		if row.providerRange == "" {
			row.providerRange = fmt.Sprintf("%03d-%03d", (result.Provider-1)*100, (result.Provider-1)*100+99)
		}
		if result.ProviderStatus != "" {
			row.providerStatus = strings.ToLower(result.ProviderStatus)
		} else {
			row.providerStatus = "-"
		}
		if result.ProviderSpeed != "" {
			row.providerSpeed = result.ProviderSpeed
		} else {
			row.providerSpeed = "-"
		}
		if result.Speed != nil {
			row.prePingMS = result.PrePingMS
			row.latencyMS = result.Speed.LatencyMS
			row.downloadMB = result.Speed.DownloadMB
			row.uploadMB = result.Speed.UploadMB
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].downloadMB != rows[j].downloadMB {
			return rows[i].downloadMB > rows[j].downloadMB
		}
		if rows[i].uploadMB != rows[j].uploadMB {
			return rows[i].uploadMB > rows[j].uploadMB
		}
		if rows[i].latencyMS != rows[j].latencyMS {
			return rows[i].latencyMS < rows[j].latencyMS
		}
		return rows[i].providerRange < rows[j].providerRange
	})
	return rows
}

func uniqueSorted(values []string) {
	seen := map[string]struct{}{}
	filtered := values[:0]
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		filtered = append(filtered, value)
	}
	sort.Strings(filtered)
	copy(values, filtered)
	for idx := range values[len(filtered):] {
		values[idx] = ""
	}
}

func promptListName(m bench.Mullvad, ctx context.Context) (string, error) {
	existing, err := m.ListCustomLists(ctx)
	if err == nil && len(existing) > 0 {
		fmt.Println("Existing lists:")
		for idx, name := range existing {
			fmt.Printf("  %d) %s\n", idx+1, name)
		}
		fmt.Println()
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Custom list name: ")
		text, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return "", errSelectionCancelled
			}
			return "", err
		}
		name := strings.TrimSpace(text)
		if name != "" {
			return name, nil
		}
		fmt.Println("list name is required")
	}
}

func provisionCustomList(ctx context.Context, m bench.Mullvad, listName string, appendMode bool) error {
	exists, err := m.CustomListExists(ctx, listName)
	if err != nil {
		return err
	}
	if !appendMode && exists {
		if err := m.DeleteCustomList(ctx, listName); err != nil {
			return err
		}
		exists = false
	}
	if !exists {
		if err := m.CreateCustomList(ctx, listName); err != nil {
			return err
		}
	}
	return nil
}

func applySelection(ctx context.Context, m bench.Mullvad, listName string, rows []selectionRow, selected []int) (int, int, error) {
	seen := map[string]struct{}{}
	added := 0
	failed := 0
	for _, idx := range selected {
		if idx < 0 || idx >= len(rows) {
			continue
		}
		row := rows[idx]
		for _, relay := range row.relays {
			key := fmt.Sprintf("%s|%s|%s", row.countryCode, row.cityCode, relay)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if err := m.AddRelayToCustomList(ctx, listName, row.countryCode, row.cityCode, relay); err != nil {
				failed++
				fmt.Fprintf(os.Stderr, "failed adding %s to %s: %v\n", relay, listName, err)
				continue
			}
			added++
		}
	}
	return added, failed, nil
}

type bucketKey struct {
	country  string
	city     string
	provider int
}

func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

var errSelectionCancelled = errors.New("selection cancelled")

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
