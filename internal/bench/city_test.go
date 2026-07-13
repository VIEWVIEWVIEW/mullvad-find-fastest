package bench

import "testing"

func TestCitiesByProvider(t *testing.T) {
	cs := Cities([]RelayPing{
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-001", IPv4: "185.1.1.1", ProviderHost: "M247", ProviderStatus: "rented", ProviderSpeed: "10 Gbps"}, MedianMS: 30, Status: "ok"},
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-099", IPv4: "185.1.1.2", ProviderHost: "M247", ProviderStatus: "rented", ProviderSpeed: "2.5 Gbps"}, MedianMS: 25, Status: "ok"},
		{Relay: Relay{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-101", IPv4: "185.1.1.3", ProviderHost: "Leaseweb", ProviderSpeed: "20 Gbps"}, MedianMS: 40, Status: "ok"},
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
	if p1.ProviderRange != "000-099" {
		t.Fatalf("expected provider range 000-099, got %q", p1.ProviderRange)
	}
	if p1.ProviderHost != "M247" {
		t.Fatalf("expected provider host M247, got %q", p1.ProviderHost)
	}
	if p1.ProviderStatus != "rented" {
		t.Fatalf("expected provider status rented, got %q", p1.ProviderStatus)
	}
	if p1.ProviderSpeed != "2.5 Gbps" {
		t.Fatalf("expected provider speed 2.5 Gbps, got %q", p1.ProviderSpeed)
	}
	if p2.ProviderRange != "100-199" {
		t.Fatalf("expected provider range 100-199, got %q", p2.ProviderRange)
	}
	if p2.ProviderHost != "Leaseweb" {
		t.Fatalf("expected provider host Leaseweb, got %q", p2.ProviderHost)
	}
	if p2.ProviderSpeed != "20 Gbps" {
		t.Fatalf("expected provider speed 20 Gbps, got %q", p2.ProviderSpeed)
	}
	if p1.RelayIP != "185.1.1.2" {
		t.Fatalf("expected relay ip 185.1.1.2, got %q", p1.RelayIP)
	}
	if p2.RelayIP != "185.1.1.3" {
		t.Fatalf("expected relay ip 185.1.1.3, got %q", p2.RelayIP)
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

func TestCitiesFromRelays(t *testing.T) {
	cities := CitiesFromRelays([]Relay{
		{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-001"},
		{CountryCode: "de", CityCode: "par", Country: "Germany", City: "Paris", Name: "de-par-wg-099"},
		{CountryCode: "de", CityCode: "fra", Country: "Germany", City: "Frankfurt", Name: "de-fra-wg-100"},
	})
	if len(cities) != 2 {
		t.Fatalf("expected 2 grouped providers, got %d", len(cities))
	}
}
