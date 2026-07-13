package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/example/mullvad-benchmark/internal/bench"
)

func main() {
	output := flag.String("output", "proxy-benchmark-results.json", "proxy benchmark JSON output path")
	proxyTargets := multiFlag{}
	flag.Var(&proxyTargets, "proxy", "proxy URL to test (repeatable), e.g. socks5://127.0.0.1:1080 or http://127.0.0.1:8080")
	baseURL := flag.String("base-url", "https://speed.cloudflare.com", "speed test base URL")
	timeout := flag.Duration("timeout", 30*time.Second, "timeout for all HTTP operations against the proxy test host")
	flag.Parse()

	if strings.TrimSpace(*baseURL) == "" {
		fatal(fmt.Errorf("--base-url cannot be empty"))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	m := bench.Mullvad{Binary: "mullvad", Timeout: 20 * time.Second}

	targets := proxyTargets
	if len(targets) == 0 {
		discovered, err := discoverMullvadProxyTargets(ctx, m)
		if err != nil {
			fatal(err)
		}
		targets = append(targets, discovered...)
		fmt.Printf("Discovered %d proxy targets from Mullvad relay list.\n", len(targets))
	}

	results := make([]proxyResult, 0, len(targets))
	for _, proxyAddress := range targets {
		result := runProxyTest(ctx, proxyAddress, strings.TrimSuffix(*baseURL, "/"), *timeout)
		results = append(results, result)
	}

	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Status != results[j].Status {
			return results[i].Status == "OK" && results[j].Status != "OK"
		}
		return results[i].DownloadMB > results[j].DownloadMB
	})

	printTable(results)
	payload := proxyBenchmarkOutput{
		Version:   1,
		CreatedAt: time.Now().UTC(),
		BaseURL:   strings.TrimSuffix(*baseURL, "/"),
		Timeout:   timeout.String(),
		Results:   results,
	}
	if err := writeJSON(*output, payload); err != nil {
		fatal(err)
	}
	if err := writeCSV(*output+".csv", results); err != nil {
		fmt.Fprintln(os.Stderr, "warning: could not write CSV:", err)
	}
}

func discoverMullvadProxyTargets(ctx context.Context, m bench.Mullvad) ([]string, error) {
	relays, err := m.ListRelays(ctx)
	if err != nil {
		return nil, err
	}
	targets := make([]string, 0, len(relays)*2)
	seen := map[string]struct{}{}
	for _, relay := range relays {
		ip := strings.TrimSpace(relay.IPv4)
		if ip == "" {
			continue
		}
		socks := fmt.Sprintf("socks5://%s:1080", ip)
		http := fmt.Sprintf("http://%s:80", ip)
		for _, target := range []string{socks, http} {
			if _, ok := seen[target]; ok {
				continue
			}
			seen[target] = struct{}{}
			targets = append(targets, target)
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("mullvad returned no usable relay IPs for proxy discovery")
	}
	return targets, nil
}

func runProxyTest(ctx context.Context, proxyAddress string, baseURL string, timeout time.Duration) proxyResult {
	address := strings.TrimSpace(proxyAddress)
	if address == "" {
		return proxyResult{
			Proxy:  proxyAddress,
			Status: "FAILED",
			Error:  "proxy URL is empty",
		}
	}

	parsed, err := url.Parse(address)
	if err != nil {
		return proxyResult{
			Proxy:  proxyAddress,
			Status: "FAILED",
			Error:  fmt.Sprintf("invalid proxy URL: %v", err),
		}
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return proxyResult{
			Proxy:       proxyAddress,
			Status:      "FAILED",
			Error:       "proxy URL must include scheme and host",
			ProxyScheme: "",
		}
	}

	if ctx.Err() != nil {
		return proxyResult{Proxy: proxyAddress, Status: "FAILED", Error: "interrupted", ProxyScheme: parsed.Scheme}
	}

	transport := &http.Transport{
		Proxy: http.ProxyURL(parsed),
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	tester := bench.Cloudflare{
		Client:  client,
		BaseURL: baseURL,
	}

	speed, err := tester.Test(ctx)
	if err != nil {
		return proxyResult{
			Proxy:       proxyAddress,
			ProxyScheme: parsed.Scheme,
			Status:      "FAILED",
			Error:       err.Error(),
		}
	}

	return proxyResult{
		Proxy:       proxyAddress,
		ProxyScheme: parsed.Scheme,
		Status:      "OK",
		LatencyMS:   speed.LatencyMS,
		DownloadMB:  speed.DownloadMB,
		UploadMB:    speed.UploadMB,
	}
}

func printTable(results []proxyResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()
	_, _ = fmt.Fprintln(w, "Idx\tProtocol\tProxy\tStatus\tLatency\tDownload (Mbps)\tUpload (Mbps)\tError")
	for i, result := range results {
		latency := "-"
		download := "-"
		upload := "-"
		if result.Status == "OK" {
			latency = fmt.Sprintf("%.0f ms", result.LatencyMS)
			download = fmt.Sprintf("%.1f", result.DownloadMB)
			upload = fmt.Sprintf("%.1f", result.UploadMB)
		}
		_, _ = fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			i+1, result.ProxyScheme, result.Proxy, result.Status, latency, download, upload, result.Error)
	}
}

func writeCSV(path string, results []proxyResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"proxy",
		"proxy_scheme",
		"status",
		"latency_ms",
		"download_mbps",
		"upload_mbps",
		"error",
	}); err != nil {
		return err
	}
	for _, result := range results {
		err = w.Write([]string{
			result.Proxy,
			result.ProxyScheme,
			result.Status,
			fmt.Sprintf("%.0f", result.LatencyMS),
			fmt.Sprintf("%.3f", result.DownloadMB),
			fmt.Sprintf("%.3f", result.UploadMB),
			result.Error,
		})
		if err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func writeJSON(path string, payload proxyBenchmarkOutput) error {
	return bench.WriteJSONAtomic(path, payload)
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

type proxyResult struct {
	Proxy       string  `json:"proxy"`
	ProxyScheme string  `json:"proxy_scheme"`
	Status      string  `json:"status"`
	LatencyMS   float64 `json:"latency_ms,omitempty"`
	DownloadMB  float64 `json:"download_mbps,omitempty"`
	UploadMB    float64 `json:"upload_mbps,omitempty"`
	Error       string  `json:"error,omitempty"`
}

type proxyBenchmarkOutput struct {
	Version   int           `json:"version"`
	CreatedAt time.Time     `json:"created_at"`
	BaseURL   string        `json:"base_url"`
	Timeout   string        `json:"timeout"`
	Results   []proxyResult `json:"results"`
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
