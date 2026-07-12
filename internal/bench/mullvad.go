package bench

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

type Mullvad struct {
	Binary  string
	Timeout time.Duration
}

func (m Mullvad) Run(ctx context.Context, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, m.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, m.Binary, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("mullvad %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func (m Mullvad) ListRelays(ctx context.Context) ([]Relay, error) {
	out, err := m.Run(ctx, "relay", "list")
	if err != nil {
		return nil, err
	}
	return ParseRelayList(out)
}
func (m Mullvad) AddSplitApp(ctx context.Context, path string) error {
	_, err := m.Run(ctx, "split-tunnel", "app", "add", path)
	return err
}
func (m Mullvad) RemoveSplitApp(ctx context.Context, path string) error {
	_, err := m.Run(ctx, "split-tunnel", "app", "remove", path)
	return err
}
func (m Mullvad) SetLocation(ctx context.Context, country, city string, relayHost ...string) error {
	args := []string{"relay", "set", "location", country, city}
	if len(relayHost) > 0 && relayHost[0] != "" {
		args = append(args, relayHost[0])
	}
	_, err := m.Run(ctx, args...)
	return err
}
func (m Mullvad) Connect(ctx context.Context) error { _, err := m.Run(ctx, "connect"); return err }
func (m Mullvad) Disconnect(ctx context.Context) error {
	_, err := m.Run(ctx, "disconnect")
	return err
}
func (m Mullvad) Status(ctx context.Context) (string, error) { return m.Run(ctx, "status", "-v") }

var relayLocationRE = regexp.MustCompile(`(?m)Relay:\s+([a-z]{2})-([a-z0-9]{3})-`)

func ParseLocation(status string) (country, city string, ok bool) {
	m := relayLocationRE.FindStringSubmatch(status)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func WaitConnected(ctx context.Context, m Mullvad, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := m.Status(ctx)
		if err == nil && strings.Contains(strings.ToLower(out), "connected") {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return fmt.Errorf("timed out waiting for Mullvad connection")
}
