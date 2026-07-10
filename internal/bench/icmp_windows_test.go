//go:build windows

package bench

import (
	"testing"
	"time"
)

func TestNativeICMPLoopback(t *testing.T) {
	d, err := Ping(make(chan struct{}), "127.0.0.1", time.Second)
	if err != nil {
		t.Fatalf("loopback ICMP failed: %v", err)
	}
	if d <= 0 {
		t.Fatalf("expected positive RTT, got %s", d)
	}
}
