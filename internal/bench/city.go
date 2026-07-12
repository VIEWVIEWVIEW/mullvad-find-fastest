package bench

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var providerRE = regexp.MustCompile(`(\d{3})$`)

type CityKey struct {
	CountryCode string
	CityCode    string
	Provider    int
}

func Cities(relays []RelayPing) []CityResult {
	groups := map[CityKey][]RelayPing{}
	for _, r := range relays {
		key := CityKey{
			CountryCode: r.CountryCode,
			CityCode:    r.CityCode,
			Provider:    ParseProvider(r.Name),
		}
		groups[key] = append(groups[key], r)
	}
	result := make([]CityResult, 0, len(groups))
	for k, rs := range groups {
		best := 0.0
		reachable := 0
		localErrors := 0
		noReply := 0
		relayName := ""
		for _, r := range rs {
			if relayName == "" {
				relayName = r.Name
			}
			if r.MedianMS > 0 {
				reachable++
			}
			if r.MedianMS > 0 && (best == 0 || r.MedianMS < best) {
				best = r.MedianMS
				relayName = r.Name
			}
			localErrors += r.LocalErrors
			noReply += r.NoReply
		}
		result = append(result, CityResult{
			CountryCode: rs[0].CountryCode,
			Country:     rs[0].Country,
			CityCode:    rs[0].CityCode,
			City:        rs[0].City,
			Provider:    k.Provider,
			RelayName:   relayName,
			RelayCount:  len(rs),
			Reachable:   reachable,
			LocalErrors: localErrors,
			NoReply:     noReply,
			PrePingMS:   best,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].PrePingMS < result[j].PrePingMS })
	return result
}

func CitiesFromRelays(relays []Relay) []CityResult {
	rs := make([]RelayPing, 0, len(relays))
	for _, r := range relays {
		rs = append(rs, RelayPing{Relay: r})
	}
	return Cities(rs)
}

func Excluded(city CityResult, exclusions []string) bool {
	for _, x := range exclusions {
		if x == city.CountryCode || x == city.CountryCode+"-"+city.CityCode || x == city.RelayName {
			return true
		}
		if x == city.ProviderCode() {
			return true
		}
		if p, ok := parseProviderFilter(x); ok && p == city.Provider {
			return true
		}
	}
	return false
}

func ParseProvider(relayName string) int {
	m := providerRE.FindStringSubmatch(relayName)
	if len(m) != 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return providerFromCode(n)
}

func parseProviderFilter(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	parts := strings.Split(strings.TrimPrefix(value, "wg-"), "-")
	if len(parts) == 1 {
		if code, err := strconv.Atoi(value); err == nil {
			return providerFromCode(code), true
		}
		return 0, false
	}
	value = parts[len(parts)-1]
	code, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return providerFromCode(code), true
}

func providerFromCode(code int) int {
	return (code / 100) + 1
}

func (c CityResult) ProviderCode() string {
	if c.Provider == 0 {
		return fmt.Sprintf("%s-%s", c.CountryCode, c.CityCode)
	}
	return fmt.Sprintf("%s-%s-%d", c.CountryCode, c.CityCode, c.Provider)
}
