package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/example/mullvad-benchmark/internal/bench"
)

func main() {
	output := flag.String("output", "mullvad-ping-results.json", "result JSON path")
	attempts := flag.Int("attempts", 3, "ICMP attempts per relay")
	timeoutMS := flag.Int("timeout-ms", 1500, "ICMP timeout")
	limit := flag.Int("limit", 0, "only probe the first N relays; zero means all relays")
	flag.Parse()
	if *attempts < 1 {
		fmt.Fprintln(os.Stderr, "attempts must be positive")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	m := bench.Mullvad{Binary: "mullvad", Timeout: 20 * time.Second}
	self, err := os.Executable()
	if err != nil {
		fatal(err)
	}
	self, err = filepath.Abs(self)
	if err != nil {
		fatal(err)
	}
	// Capture the current setting for diagnostics; cleanup removes only this program's entry.
	_, _ = m.Run(ctx, "split-tunnel", "get")
	if err := m.AddSplitApp(ctx, self); err != nil {
		fatal(err)
	}
	logf("split tunnel: added %s", self)
	defer func() {
		if err := m.RemoveSplitApp(context.Background(), self); err != nil {
			fmt.Fprintln(os.Stderr, "warning: could not remove split-tunnel entry:", err)
		} else {
			logf("split tunnel: removed %s", self)
		}
	}()
	if info, err := bench.LookupPublicIP(ctx, &http.Client{Timeout: 15 * time.Second}); err != nil {
		logf("public IP lookup failed: %v", err)
	} else {
		logf("public IP: ip=%s hostname=%s location=%s", info.IP, info.Hostname, info.Location)
	}
	relays, err := m.ListRelays(ctx)
	if err != nil {
		fatal(err)
	}
	if *limit > 0 && *limit < len(relays) {
		relays = relays[:*limit]
		logf("probe limit: testing first %d relay(s)", len(relays))
	}
	f := bench.PingFile{Version: 1, RunID: fmt.Sprintf("%d", time.Now().UnixNano()), CreatedAt: time.Now().UTC(), Settings: bench.PingConfig{Attempts: *attempts, JitterMinMS: 5, JitterMaxMS: 25, TimeoutMS: *timeoutMS}}
	f.Relays = make([]bench.RelayPing, len(relays))
	for i, relay := range relays {
		p := bench.RelayPing{Relay: relay}
		for n := 0; n < *attempts; n++ {
			select {
			case <-ctx.Done():
				p.Error = ctx.Err().Error()
				n = *attempts
				continue
			default:
			}
			time.Sleep(time.Duration(5+rand.Intn(21)) * time.Millisecond)
			attempt := n + 1
			started := time.Now()
			d, err := bench.Ping(ctx.Done(), relay.IPv4, time.Duration(*timeoutMS)*time.Millisecond)
			if err != nil {
				p.Failures++
				result := "local_error"
				if errors.Is(err, bench.ErrICMPNoReply) {
					p.NoReply++
					result = "no_reply"
				} else {
					p.LocalErrors++
				}
				logf("ping %s/%s relay=%s ip=%s attempt=%d/%d result=%s elapsed=%s error=%v", relay.CountryCode, relay.CityCode, relay.Name, relay.IPv4, attempt, *attempts, result, time.Since(started).Round(time.Millisecond), err)
			} else {
				p.Samples = append(p.Samples, d.Milliseconds())
				logf("ping %s/%s relay=%s ip=%s attempt=%d/%d result=ok rtt=%dms elapsed=%s", relay.CountryCode, relay.CityCode, relay.Name, relay.IPv4, attempt, *attempts, d.Milliseconds(), time.Since(started).Round(time.Millisecond))
			}
		}
		p.MedianMS = bench.Median(p.Samples)
		p.MinMS, p.MaxMS = bench.MinMax(p.Samples)
		if len(p.Samples) > 0 {
			p.Status = "ok"
		} else if p.LocalErrors > 0 {
			p.Status = "local_error"
		} else if p.NoReply == *attempts {
			p.Status = "no_reply"
		} else {
			p.Status = "error"
		}
		f.Relays[i] = p
	}
	if err := bench.WriteJSONAtomic(*output, f); err != nil {
		fatal(err)
	}
	failures := 0
	for _, r := range f.Relays {
		failures += r.Failures
	}
	fmt.Printf("Wrote %d relay results to %s (%d failed probes)\n", len(f.Relays), *output, failures)
}

func fatal(err error) { fmt.Fprintln(os.Stderr, "error:", err); os.Exit(1) }

func logf(format string, args ...any) {
	fmt.Printf("[%s] %s\n", time.Now().Format("15:04:05.000"), fmt.Sprintf(format, args...))
}
