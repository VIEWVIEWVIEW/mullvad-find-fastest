package bench

import "testing"

func TestCitiesAndExcluded(t *testing.T) {
	cs := Cities([]RelayPing{{Relay: Relay{CountryCode: "de", CityCode: "fra", Country: "Germany", City: "Frankfurt"}, MedianMS: 0, Status: "no_reply"}, {Relay: Relay{CountryCode: "de", CityCode: "fra"}, MedianMS: 30, Status: "ok"}})
	if len(cs) != 1 || cs[0].PrePingMS != 30 || cs[0].Reachable != 1 || !Excluded(cs[0], []string{"de-fra"}) {
		t.Fatalf("unexpected %#v", cs)
	}
}
