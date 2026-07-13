//go:build !windows

package main

import (
	"bufio"
	"io"
	"os"
	"strconv"
)

func enableRawMode() (func(), error) {
	return nil, io.EOF
}

func readKey(reader *bufio.Reader) (keyCode, error) {
	ch, err := reader.ReadByte()
	if err != nil {
		return keyOther, err
	}
	switch ch {
	case '\r', '\n':
		return keyEnter, nil
	case 'k', 'K':
		return keyUp, nil
	case 'j', 'J':
		return keyDown, nil
	case 3:
		return keyCtrlC, nil
	case 'q', 'Q':
		return keyQuit, nil
	case ' ':
		return keySpace, nil
	default:
		return keyOther, nil
	}
}

func terminalHeight() int {
	if lines := os.Getenv("LINES"); lines != "" {
		if value, err := strconv.Atoi(lines); err == nil && value > 0 {
			return value
		}
	}
	return 24
}
