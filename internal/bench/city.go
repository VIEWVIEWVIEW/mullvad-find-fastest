package bench

import "sort"

type CityKey struct{ CountryCode, CityCode string }

func Cities(relays []RelayPing) []CityResult {
	groups := map[CityKey][]RelayPing{}
	for _, r := range relays {
		groups[CityKey{r.CountryCode, r.CityCode}] = append(groups[CityKey{r.CountryCode, r.CityCode}], r)
	}
	result := make([]CityResult, 0, len(groups))
	for _, rs := range groups {
		best := 0.0
		reachable := 0
		for _, r := range rs {
			if r.MedianMS > 0 {
				reachable++
			}
			if r.MedianMS > 0 && (best == 0 || r.MedianMS < best) {
				best = r.MedianMS
			}
		}
		result = append(result, CityResult{CountryCode: rs[0].CountryCode, Country: rs[0].Country, CityCode: rs[0].CityCode, City: rs[0].City, RelayCount: len(rs), Reachable: reachable, PrePingMS: best})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PrePingMS < result[j].PrePingMS })
	return result
}

func Excluded(city CityResult, exclusions []string) bool {
	for _, x := range exclusions {
		if x == city.CountryCode || x == city.CountryCode+"-"+city.CityCode {
			return true
		}
	}
	return false
}
