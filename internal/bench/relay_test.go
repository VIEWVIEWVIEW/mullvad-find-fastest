package bench

import "testing"

func TestParseRelayList(t *testing.T) {
	input := "Germany (de)\n\tFrankfurt (fra) @ 50N, 8E\n\t\tde-fra-wg-001 (185.213.155.73, 2a::1) - hosted\n"
	got, err := ParseRelayList(input)
	if err != nil || len(got) != 1 {
		t.Fatalf("err=%v relays=%d", err, len(got))
	}
	if got[0].CountryCode != "de" || got[0].CityCode != "fra" || got[0].IPv4 != "185.213.155.73" {
		t.Fatalf("unexpected %#v", got[0])
	}
}
