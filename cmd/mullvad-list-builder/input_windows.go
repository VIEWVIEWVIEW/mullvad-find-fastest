//go:build windows

package main

import (
	"bufio"
	"syscall"
	"unsafe"
)

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleMode = kernel32.NewProc("GetConsoleMode")
	procSetConsoleMode = kernel32.NewProc("SetConsoleMode")
)

const (
	enableProcessedInput = 0x0001
	enableLineInput      = 0x0002
	enableEchoInput      = 0x0004
	enableWindowInput    = 0x0008
	enableMouseInput     = 0x0010
	enableExtendedFlags  = 0x0080
	enableVirtualInput   = 0x0200
)

func enableRawMode() (func(), error) {
	stdin, err := syscall.GetStdHandle(syscall.STD_INPUT_HANDLE)
	if err != nil {
		return nil, err
	}
	var mode uint32
	if _, _, err := procGetConsoleMode.Call(uintptr(stdin), uintptr(unsafe.Pointer(&mode))); err != nil {
		return nil, err
	}

	raw := mode &
		^uint32(enableLineInput|enableEchoInput|enableProcessedInput)
	raw |= uint32(enableWindowInput | enableMouseInput | enableExtendedFlags | enableVirtualInput)
	if _, _, err := procSetConsoleMode.Call(uintptr(stdin), uintptr(raw)); err != nil {
		return nil, err
	}

	return func() {
		_, _, _ = procSetConsoleMode.Call(uintptr(stdin), uintptr(mode))
	}, nil
}

func readKey(reader *bufio.Reader) (keyCode, error) {
	b, err := reader.ReadByte()
	if err != nil {
		return keyOther, err
	}
	switch b {
	case 0:
		next, err := reader.ReadByte()
		if err != nil {
			return keyOther, err
		}
		switch next {
		case 'H':
			return keyUp, nil
		case 'P':
			return keyDown, nil
		case 'K':
			return keyLeft, nil
		case 'M':
			return keyRight, nil
		case 'G':
			return keyHome, nil
		case 'O':
			return keyEnd, nil
		}
	case 0x1b:
		next, err := reader.ReadByte()
		if err != nil {
			return keyOther, err
		}
		if next == '[' {
			arrow, err := reader.ReadByte()
			if err != nil {
				return keyOther, err
			}
			switch arrow {
			case 'A':
				return keyUp, nil
			case 'B':
				return keyDown, nil
			case 'D':
				return keyLeft, nil
			case 'C':
				return keyRight, nil
			case 'H':
				return keyHome, nil
			case 'F':
				return keyEnd, nil
			default:
				return keyOther, nil
			}
		}
	case ' ':
		return keySpace, nil
	case '\r', '\n':
		return keyEnter, nil
	case 'a', 'A':
		return keyAll, nil
	case 'c', 'C':
		return keyClear, nil
	case 'q', 'Q':
		return keyQuit, nil
	case 3:
		return keyCtrlC, nil
	}
	return keyOther, nil
}
