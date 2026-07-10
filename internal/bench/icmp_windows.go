//go:build windows

package bench

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"
)

var (
	ErrICMPResources = errors.New("ICMP resources exhausted")
	ErrICMPNoReply   = errors.New("no ICMP reply")
)

var (
	iphlpapi     = syscall.NewLazyDLL("iphlpapi.dll")
	icmpCreate   = iphlpapi.NewProc("IcmpCreateFile")
	icmpClose    = iphlpapi.NewProc("IcmpCloseHandle")
	icmpSendEcho = iphlpapi.NewProc("IcmpSendEcho")
)

type icmpOptions struct {
	TTL, TOS, Flags, OptionsSize byte
	OptionsData                  uintptr
}
type icmpReply struct {
	Address, Status, RoundTripTime uint32
	DataSize, Reserved             uint16
	DataPointer                    uintptr
	Options                        icmpOptions
}

func Ping(ctxDone <-chan struct{}, address string, timeout time.Duration) (time.Duration, error) {
	ip := net.ParseIP(address).To4()
	if ip == nil {
		return 0, fmt.Errorf("not an IPv4 address: %s", address)
	}
	select {
	case <-ctxDone:
		return 0, ctxError{}
	default:
	}
	for attempt := 0; attempt < 3; attempt++ {
		d, err := pingOnce(ip, timeout)
		if !errors.Is(err, ErrICMPResources) || attempt == 2 {
			return d, err
		}
		select {
		case <-ctxDone:
			return 0, ctxError{}
		case <-time.After(time.Duration(50*(attempt+1)) * time.Millisecond):
		}
	}
	return 0, ErrICMPResources
}

func pingOnce(ip net.IP, timeout time.Duration) (time.Duration, error) {
	h, _, err := icmpCreate.Call()
	if h == 0 || h == ^uintptr(0) {
		return 0, classifyICMPError(err)
	}
	defer icmpClose.Call(h)
	data := []byte("mullvad-benchmark")
	reply := make([]byte, 256)
	start := time.Now()
	// IcmpSendEcho has exactly eight parameters. Passing an extra argument
	// corrupts the native call contract on Windows.
	n, _, callErr := icmpSendEcho.Call(
		h,
		uintptr(binary.BigEndian.Uint32(ip)),
		uintptr(unsafe.Pointer(&data[0])),
		uintptr(len(data)),
		0,
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(timeout.Milliseconds()),
	)
	if n == 0 {
		// On some Windows/VPN routing combinations, IcmpSendEcho reports
		// ERROR_NOT_ENOUGH_MEMORY after waiting for the full timeout when the
		// destination simply did not answer. Treat delayed failures as packet
		// loss; reserve local_resource for immediate API failures.
		if time.Since(start) >= timeout*8/10 {
			return 0, fmt.Errorf("%w: %v", ErrICMPNoReply, callErr)
		}
		return 0, classifyICMPError(callErr)
	}
	r := (*icmpReply)(unsafe.Pointer(&reply[0]))
	if r.Status != 0 {
		return 0, fmt.Errorf("%w: ICMP status %d", ErrICMPNoReply, r.Status)
	}
	return time.Since(start), nil
}

func classifyICMPError(err error) error {
	if err == nil {
		return ErrICMPNoReply
	}
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.Errno(8), syscall.Errno(11010):
			return fmt.Errorf("%w: %v", ErrICMPResources, err)
		case syscall.Errno(1460):
			return fmt.Errorf("%w: %v", ErrICMPNoReply, err)
		}
	}
	return err
}

type ctxError struct{}

func (ctxError) Error() string { return "ping cancelled" }
