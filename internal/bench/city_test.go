package bench

import "testing"

func TestCitiesByProvider(t *testing.T) {
	cs := Cities([]RelayPing{
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-001"}, MedianMS: 30, Status: "ok"},
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-099"}, MedianMS: 25, Status: "ok"},
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-101"}, MedianMS: 40, Status: "ok"},
	})
	if len(cs) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(cs))
	}
	p1 := CityResult{}
	p2 := CityResult{}
	for _, c := range cs {
		if c.Provider == 1 {
			p1 = c
		}
		if c.Provider == 2 {
			p2 = c
		}
	}
	if p1.RelayName != "de-par-wg-099" || p1.RelayCount != 2 || p1.Reachable != 2 || p1.PrePingMS != 25 {
		t.Fatalf("unexpected provider1 bucket %#v", p1)
	}
	if p2.RelayName != "de-par-wg-101" || p2.PrePingMS != 40 {
		t.Fatalf("unexpected provider2 bucket %#v", p2)
	}
	if !Excluded(p1, []string{"de-par", "de-par-1", "de-par-wg-099", "de-par-wg-001"}) {
		t.Fatal("expected provider exclusion to match")
	}
}

func TestCitiesSelectionSkipsOnlyNoReplyWhenUnreachable(t *testing.T) {
	relays := []RelayPing{
		{
			Relay:       Relay{CountryCode: "de", CityCode: "fra", Country: "Germany", City: "Frankfurt", Name: "de-fra-wg-001"},
			NoReply:     3,
			Status:      "no_reply",
			MedianMS:    0,
			LocalErrors: 0,
		},
		{
			Relay:       Relay{CountryCode: "de", CityCode: "fra", Country: "Germany", City: "Frankfurt", Name: "de-fra-wg-101"},
			LocalErrors: 3,
			Status:      "local_error",
			MedianMS:    0,
		},
	}
	cityResults := Cities(relays)
	if len(cityResults) != 2 {
		t.Fatalf("expected 2 provider buckets, got %d", len(cityResults))
	}

	var noReplyProvider, localErrorProvider CityResult
	for _, r := range cityResults {
		if r.RelayName == "de-fra-wg-001" {
			noReplyProvider = r
		}
		if r.RelayName == "de-fra-wg-101" {
			localErrorProvider = r
		}
	}
	if noReplyProvider.NoReply == 0 || noReplyProvider.Reachable != 0 {
		t.Fatalf("unexpected no-reply aggregation %#v", noReplyProvider)
	}
	if localErrorProvider.LocalErrors != 3 || localErrorProvider.Reachable != 0 {
		t.Fatalf("unexpected local-error aggregation %#v", localErrorProvider)
	}
}
