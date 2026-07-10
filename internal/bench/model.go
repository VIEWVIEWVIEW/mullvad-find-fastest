package bench

import "time"

type Relay struct {
	CountryCode string `json:"country_code"`
	Country     string `json:"country"`
	CityCode    string `json:"city_code"`
	City        string `json:"city"`
	Name        string `json:"name"`
	IPv4        string `json:"ipv4"`
}

type RelayPing struct {
	Relay
	Samples     []int64 `json:"samples_ms"`
	MedianMS    float64 `json:"median_ms"`
	MinMS       float64 `json:"min_ms"`
	MaxMS       float64 `json:"max_ms"`
	Failures    int     `json:"failures"`
	NoReply     int     `json:"no_reply"`
	LocalErrors int     `json:"local_errors"`
	Status      string  `json:"status"`
	Error       string  `json:"error,omitempty"`
}

type PingFile struct {
	Version   int         `json:"version"`
	RunID     string      `json:"run_id"`
	CreatedAt time.Time   `json:"created_at"`
	Settings  PingConfig  `json:"settings"`
	Relays    []RelayPing `json:"relays"`
}

type PingConfig struct {
	Attempts    int `json:"attempts"`
	JitterMinMS int `json:"jitter_min_ms"`
	JitterMaxMS int `json:"jitter_max_ms"`
	TimeoutMS   int `json:"timeout_ms"`
}

type SpeedResult struct {
	LatencyMS  float64 `json:"latency_ms"`
	DownloadMB float64 `json:"download_mbps"`
	UploadMB   float64 `json:"upload_mbps"`
	Error      string  `json:"error,omitempty"`
}

type CityResult struct {
	CountryCode string       `json:"country_code"`
	Country     string       `json:"country"`
	CityCode    string       `json:"city_code"`
	City        string       `json:"city"`
	RelayCount  int          `json:"relay_count"`
	Reachable   int          `json:"reachable_relays"`
	PrePingMS   float64      `json:"pre_ping_ms"`
	Speed       *SpeedResult `json:"speed,omitempty"`
	Status      string       `json:"status"`
	Error       string       `json:"error,omitempty"`
}

type BenchmarkFile struct {
	Version   int          `json:"version"`
	RunID     string       `json:"run_id"`
	CreatedAt time.Time    `json:"created_at"`
	InputRun  string       `json:"input_run"`
	Results   []CityResult `json:"results"`
}
