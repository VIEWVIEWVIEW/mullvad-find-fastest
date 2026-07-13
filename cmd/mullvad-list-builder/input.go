package main

import "errors"

type keyCode int

const (
	keyUp keyCode = iota
	keyDown
	keyLeft
	keyRight
	keyHome
	keyEnd
	keySpace
	keyEnter
	keyQuit
	keyCtrlC
	keyAll
	keyClear
	keyOther
)

var errSelectionRequiresFallback = errors.New("selection requires fallback mode")
