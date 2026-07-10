package bench

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var (
	countryRE = regexp.MustCompile(`^(.+) \(([a-z]{2})\)$`)
	cityRE    = regexp.MustCompile(`^(.+) \(([a-z0-9]{3})\) @`)
	relayRE   = regexp.MustCompile(`^([a-z]{2})-([a-z0-9]{3})-[^ ]+ \(([^, )]+)(?:, [^)]*)?\)`)
)

func ParseRelayList(text string) ([]Relay, error) {
	var country, countryCode, city, cityCode string
	var relays []Relay
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if m := countryRE.FindStringSubmatch(line); m != nil && !strings.Contains(line, " @") {
			country, countryCode = m[1], m[2]
			city, cityCode = "", ""
			continue
		}
		if m := cityRE.FindStringSubmatch(line); m != nil {
			city, cityCode = m[1], m[2]
			continue
		}
		if m := relayRE.FindStringSubmatch(line); m != nil && countryCode != "" && cityCode != "" {
			relays = append(relays, Relay{CountryCode: countryCode, Country: country, CityCode: cityCode, City: city, Name: strings.Split(line, " ")[0], IPv4: m[3]})
		}
	}
	if len(relays) == 0 {
		return nil, fmt.Errorf("no relays parsed from mullvad output")
	}
	sort.Slice(relays, func(i, j int) bool { return relays[i].Name < relays[j].Name })
	return relays, nil
}
