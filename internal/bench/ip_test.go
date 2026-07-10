package bench

import (
	"encoding/json"
	"testing"
)

func TestPublicIPInfoJSON(t *testing.T) {
	var info PublicIPInfo
	err := json.Unmarshal([]byte(`{"YourFuckingIPAddress":"203.0.113.8","YourFuckingLocation":"Berlin, Germany","YourFuckingHostname":"example"}`), &info)
	if err != nil || info.IP != "203.0.113.8" || info.Location != "Berlin, Germany" || info.Hostname != "example" {
		t.Fatalf("unexpected %#v: %v", info, err)
	}
}
