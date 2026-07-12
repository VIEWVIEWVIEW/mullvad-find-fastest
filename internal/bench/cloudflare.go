package bench

import (
	"bytes"
	"context"
	"errors"
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

var (
	defaultDownloadCandidates = []int64{
		50 * 1024 * 1024,
		25 * 1024 * 1024,
		20 * 1024 * 1024,
		10 * 1024 * 1024,
		5 * 1024 * 1024,
	}
	defaultDownloadTimeout = 20 * time.Second
)

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
	downloadBytes := c.maxAvailableDownloadBytes(ctx)
	download, err := c.transferDownload(ctx, fmt.Sprintf("__down?bytes=%d", downloadBytes), defaultDownloadTimeout)
	if err != nil {
		return nil, fmt.Errorf("download: %w", err)
	}
	upload, err := c.transfer(ctx, "__up", http.MethodPost, make([]byte, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("upload: %w", err)
	}
	return &SpeedResult{LatencyMS: Median(latency), DownloadMB: download, UploadMB: upload}, nil
}

func (c Cloudflare) maxAvailableDownloadBytes(ctx context.Context) int64 {
	for _, candidate := range defaultDownloadCandidates {
		if candidate <= 0 {
			continue
		}
		if c.isDownloadAvailable(ctx, candidate) {
			return candidate
		}
	}
	return 5 * 1024 * 1024
}

func (c Cloudflare) isDownloadAvailable(ctx context.Context, bytes int64) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.BaseURL+"/"+("__down?bytes="+strconv.FormatInt(bytes, 10)), nil)
	if err != nil {
		return false
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return false
	}
	if cl, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
		if cl > 0 && cl < bytes {
			return false
		}
	}
	return true
}

func (c Cloudflare) transferDownload(ctx context.Context, path string, timeout time.Duration) (float64, error) {
	downloadCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, c.BaseURL+"/"+path, nil)
	if err != nil {
		return 0, err
	}
	start := time.Now()
	resp, err := c.Client.Do(req)
	if err != nil {
		if isTimeoutError(downloadCtx, err) {
			return 0, fmt.Errorf("download timed out")
		}
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	responseBytes, copyErr := io.Copy(io.Discard, resp.Body)
	elapsed := time.Since(start)
	if copyErr != nil {
		if isTimeoutError(downloadCtx, copyErr) {
			if responseBytes == 0 {
				return 0, nil
			}
			return float64(responseBytes*8) / elapsed.Seconds() / 1e6, nil
		}
		return 0, copyErr
	}
	return float64(responseBytes*8) / elapsed.Seconds() / 1e6, nil
}

func isTimeoutError(ctx context.Context, err error) bool {
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if ctx == nil {
		return false
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return true
	}
	if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
		return true
	}
	return false
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
	var requestBytes int64
	if body != nil {
		reqBody = bytes.NewReader(body)
		requestBytes = int64(len(body))
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
	responseBytes, err := io.Copy(io.Discard, resp.Body)
	if err != nil {
		return 0, err
	}
	if resp.StatusCode/100 != 2 {
		return 0, fmt.Errorf("HTTP %s", resp.Status)
	}
	seconds := time.Since(start).Seconds()
	transferredBytes := responseBytes
	if body != nil {
		transferredBytes = requestBytes
	}
	return float64(transferredBytes*8) / seconds / 1e6, nil
}
