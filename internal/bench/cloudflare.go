package bench

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type Cloudflare struct {
	Client  *http.Client
	BaseURL string
	Timeout time.Duration
}

func (c Cloudflare) Test(ctx context.Context) (*SpeedResult, error) {
	if c.Client == nil {
		c.Client = &http.Client{Timeout: c.Timeout}
	}
	if c.BaseURL == "" {
		c.BaseURL = "https://speed.cloudflare.com"
	}
	var latency []int64
	for i := 0; i < 5; i++ {
		d, err := c.request(ctx, "__down?bytes=0", nil)
		if err != nil {
			return nil, fmt.Errorf("latency: %w", err)
		}
		latency = append(latency, d.Milliseconds())
	}
	download, err := c.transfer(ctx, "__down?bytes=5242880", http.MethodGet, nil)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	upload, err := c.transfer(ctx, "__up", http.MethodPost, make([]byte, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	return &SpeedResult{LatencyMS: Median(latency), DownloadMB: download, UploadMB: upload}, nil
}

func (c Cloudflare) request(ctx context.Context, path string, body io.Reader) (time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/"+path, body)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return time.Since(start), nil
}

func (c Cloudflare) transfer(ctx context.Context, path, method string, body []byte) (float64, error) {
	var reqBody io.Reader
	if body != nil {
		reqBody = io.LimitReader(bytesReader(body), int64(len(body)))
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+"/"+path, reqBody)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.ContentLength = int64(len(body))
	}
	start := time.Now()
	resp, err := c.Client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	n, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	seconds := time.Since(start).Seconds()
	return float64(n*8) / seconds / 1e6, nil
}

type byteReader struct {
	b   []byte
	pos int
}

func bytesReader(b []byte) io.Reader { return &byteReader{b: b} }
func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos == len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}
func parseBytes(s string) int { n, _ := strconv.Atoi(s); return n }
