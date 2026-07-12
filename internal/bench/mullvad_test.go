package bench

import (
	"testing"
)

func TestParseLocation(t *testing.T) {
	country, city, ok := ParseLocation("Connected\n    Relay: de-ber-wg-003 (193.32.248.68:51820/UDP) via gb-lon-wg-308")
	if !ok || country != "de" || city != "ber" {
		t.Fatalf("got %q %q %v", country, city, ok)
	}
}

func TestParseRelayMultihopState(t *testing.T) {
	enabled, err := parseRelayMultihopState("WireGuard options\n    MTU: unset\nMultihop state: enabled")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !enabled {
		t.Fatal("expected multihop enabled")
	}
	disabled, err := parseRelayMultihopState("Multihop state:    off")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if disabled {
		t.Fatal("expected multihop disabled")
	}
}
