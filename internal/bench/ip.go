package bench

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type PublicIPInfo struct {
	IP       string `json:"YourFuckingIPAddress"`
	Location string `json:"YourFuckingLocation"`
	Hostname string `json:"YourFuckingHostname"`
}

func LookupPublicIP(ctx context.Context, client *http.Client) (PublicIPInfo, error) {
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://wtfismyip.com/json", nil)
	if err != nil {
		return PublicIPInfo{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return PublicIPInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return PublicIPInfo{}, fmt.Errorf("HTTP %s", resp.Status)
	}
	var info PublicIPInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return PublicIPInfo{}, err
	}
	return info, nil
}
