package bench

import "testing"

func TestParseRelayList(t *testing.T) {
	input := "Germany (de)\n\tFrankfurt (fra) @ 50N, 8E\n\t\tde-fra-wg-001 (185.213.155.73, 2a::1) - hosted by M247 (rented), 10 Gbps\n"
	got, err := ParseRelayList(input)
	if err != nil || len(got) != 1 {
		t.Fatalf("err=%v relays=%d", err, len(got))
	}
	if got[0].CountryCode != "de" || got[0].CityCode != "fra" || got[0].IPv4 != "185.213.155.73" {
		t.Fatalf("unexpected %#v", got[0])
	}
	if got[0].Provider != 1 {
		t.Fatalf("expected provider 1, got %d", got[0].Provider)
	}
	if got[0].ProviderHost != "M247" {
		t.Fatalf("expected provider host M247, got %q", got[0].ProviderHost)
	}
	if got[0].ProviderStatus != "rented" {
		t.Fatalf("expected provider status rented, got %q", got[0].ProviderStatus)
	}
	if got[0].ProviderSpeed != "10 Gbps" {
		t.Fatalf("expected provider speed 10 Gbps, got %q", got[0].ProviderSpeed)
	}
}
