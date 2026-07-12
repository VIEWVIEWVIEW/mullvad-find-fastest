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

func TestParseRelayIdentity(t *testing.T) {
	country, city, relay, ok := ParseRelayIdentity("Connected\n    Relay: al-tia-wg-003 (103.124.165.130:51820/UDP) via gb-lon-wg-308")
	if !ok || country != "al" || city != "tia" || relay != "al-tia-wg-003" {
		t.Fatalf("got %q %q %q %v", country, city, relay, ok)
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
