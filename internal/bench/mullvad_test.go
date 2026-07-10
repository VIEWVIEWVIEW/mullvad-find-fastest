package bench

import "testing"

func TestParseLocation(t *testing.T) {
	country, city, ok := ParseLocation("Connected\n    Relay: de-ber-wg-003 (193.32.248.68:51820/UDP) via gb-lon-wg-308")
	if !ok || country != "de" || city != "ber" {
		t.Fatalf("got %q %q %v", country, city, ok)
	}
}
