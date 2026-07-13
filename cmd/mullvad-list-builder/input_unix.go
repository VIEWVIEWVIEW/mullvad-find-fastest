//go:build !windows

package main

import (
	"bufio"
	"io"
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
	case 3:
		return keyCtrlC, nil
	case 'q', 'Q':
		return keyQuit, nil
	case 'a', 'A':
		return keyAll, nil
	case 'c', 'C':
		return keyClear, nil
	case ' ':
		return keySpace, nil
	default:
		return keyOther, nil
	}
}
