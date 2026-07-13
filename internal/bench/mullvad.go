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

func (m Mullvad) ListCustomLists(ctx context.Context) ([]string, error) {
	out, err := m.Run(ctx, "custom-list", "list")
	if err != nil {
		return nil, err
	}
	return ParseCustomListNames(out), nil
}

func (m Mullvad) CustomListExists(ctx context.Context, name string) (bool, error) {
	lists, err := m.ListCustomLists(ctx)
	if err != nil {
		return false, err
	}
	for _, existing := range lists {
		if strings.EqualFold(existing, name) {
			return true, nil
		}
	}
	return false, nil
}

func (m Mullvad) CreateCustomList(ctx context.Context, name string) error {
	_, err := m.Run(ctx, "custom-list", "new", name)
	return err
}

func (m Mullvad) DeleteCustomList(ctx context.Context, name string) error {
	_, err := m.Run(ctx, "custom-list", "delete", name)
	return err
}

func (m Mullvad) AddRelayToCustomList(ctx context.Context, listName, country, city, hostname string) error {
	_, err := m.Run(ctx, "custom-list", "edit", "add", listName, country, city, hostname)
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
func (m Mullvad) IsMultihopEnabled(ctx context.Context) (bool, error) {
	out, err := m.Run(ctx, "relay", "get")
	if err != nil {
		return false, err
	}
	return parseRelayMultihopState(out)
}
func (m Mullvad) SetMultihop(ctx context.Context, enabled bool) error {
	state := "off"
	if enabled {
		state = "on"
	}
	_, err := m.Run(ctx, "relay", "set", "multihop", state)
	return err
}

var relayLocationRE = regexp.MustCompile(`(?m)Relay:\s+([a-z]{2})-([a-z0-9]{3})-`)
var relayIdentityRE = regexp.MustCompile(`(?m)Relay:\s+([a-z]{2}-[a-z0-9]{3}-[^\s]+)\b`)
var relayMultihopStateRE = regexp.MustCompile(`(?m)^\s*Multihop state:\s+([A-Za-z]+)\s*$`)

func ParseCustomListNames(text string) []string {
	var names []string
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimRight(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			continue
		}
		if strings.Contains(line, ",") && strings.Contains(line, "(") && strings.Contains(line, ")") {
			continue
		}
		names = append(names, strings.TrimSpace(line))
	}
	return names
}

func ParseLocation(status string) (country, city string, ok bool) {
	m := relayLocationRE.FindStringSubmatch(status)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

func ParseRelayIdentity(status string) (country, city, relay string, ok bool) {
	m := relayLocationRE.FindStringSubmatch(status)
	if m == nil {
		return "", "", "", false
	}
	mRelay := relayIdentityRE.FindStringSubmatch(status)
	if mRelay == nil || len(mRelay) != 2 {
		return m[1], m[2], "", true
	}
	return m[1], m[2], mRelay[1], true
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

func parseRelayMultihopState(out string) (bool, error) {
	m := relayMultihopStateRE.FindStringSubmatch(out)
	if m == nil {
		return false, fmt.Errorf("unable to read multihop state from relay output")
	}
	switch strings.ToLower(m[1]) {
	case "enabled", "on":
		return true, nil
	case "disabled", "off":
		return false, nil
	default:
		return false, fmt.Errorf("unknown multihop state: %s", m[1])
	}
}
