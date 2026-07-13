package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
)

const listHeaderLines = 5

func promptForSelection(rows []selectionRow) ([]int, error) {
	selected, err := runInteractiveSelector(rows)
	return selected, err
}

func runInteractiveSelector(rows []selectionRow) ([]int, error) {
	state := selectorState{
		rows:     rows,
		selected: map[int]struct{}{},
		cursor:   0,
		offset:   0,
	}

	reader := bufio.NewReaderSize(os.Stdin, 1)
	restore, err := enableRawMode()
	if err != nil {
		return nil, err
	}
	defer restore()

	if len(state.rows) == 0 {
		return nil, nil
	}

	for {
		updateOffset(&state)
		renderState(state)
		key, err := readKey(reader)
		if err != nil {
			return nil, err
		}
		switch key {
		case keyQuit, keyCtrlC:
			return nil, errSelectionCancelled
		case keyUp:
			if state.cursor > 0 {
				state.cursor--
			}
		case keyDown:
			if state.cursor < len(state.rows)-1 {
				state.cursor++
			}
		case keyHome:
			state.cursor = 0
		case keyEnd:
			state.cursor = len(state.rows) - 1
		case keySpace:
			if _, ok := state.selected[state.cursor]; ok {
				delete(state.selected, state.cursor)
			} else {
				state.selected[state.cursor] = struct{}{}
			}
		case keyEnter:
			return orderedSelection(state.selected), nil
		}
	}
}

type selectorState struct {
	cursor   int
	rows     []selectionRow
	selected map[int]struct{}
	offset   int
}

func orderedSelection(selected map[int]struct{}) []int {
	out := make([]int, 0, len(selected))
	for idx := range selected {
		out = append(out, idx)
	}
	sort.Ints(out)
	return out
}

func renderState(state selectorState) {
	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("Choose providers (city + provider) to add.")
	fmt.Println("  Use up/down arrows or j/k, Space to select/deselect, Enter to submit, Q to quit")
	fmt.Printf("  Selected %d / %d\n", len(state.selected), len(state.rows))
	windowRows := maxInt(terminalListRows(), 1)
	end := minInt(state.offset+windowRows, len(state.rows))
	fmt.Printf("  Showing rows %d-%d\n\n", state.offset+1, end)
	renderRows(state.rows, state.cursor, state.offset, windowRows, state.selected)
}

func renderRows(rows []selectionRow, cursor int, offset int, windowRows int, selected map[int]struct{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	_, _ = fmt.Fprintln(w, "Idx\tSel\tCountry\tCity\tProvider\tHost\tStatus\tSpeed\tRelays\tPre ping\tLatency\tDownload\tUpload\tLocation")
	last := minInt(offset+windowRows, len(rows))
	for i := offset; i < last; i++ {
		row := rows[i]
		cursorMark := " "
		if i == cursor {
			cursorMark = ">"
		}
		checkMark := " "
		if _, ok := selected[i]; ok {
			checkMark = "*"
		}
		prePing := formatMS(row.prePingMS)
		lat := formatMS(row.latencyMS)
		download := formatSpeed(row.downloadMB)
		upload := formatSpeed(row.uploadMB)
		relays := "0"
		if row.relayCount > 0 {
			relays = fmt.Sprintf("%d", row.relayCount)
		}
		location := row.cityName
		if row.countryName != "" {
			location = fmt.Sprintf("%s (%s)", row.cityName, row.countryName)
		}

		_, _ = fmt.Fprintf(w, "%s%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			cursorMark,
			i+1,
			checkMark,
			strings.ToUpper(row.countryCode),
			row.cityCode,
			row.providerRange,
			row.providerHost,
			row.providerStatus,
			row.providerSpeed,
			relays,
			prePing,
			lat,
			download,
			upload,
			location,
		)
	}
}

func updateOffset(state *selectorState) {
	if len(state.rows) == 0 {
		state.offset = 0
		return
	}

	windowRows := maxInt(terminalListRows(), 1)
	if state.cursor < state.offset {
		state.offset = state.cursor
	}
	if state.cursor >= state.offset+windowRows {
		state.offset = state.cursor - windowRows + 1
	}
	maxOffset := len(state.rows) - windowRows
	if maxOffset < 0 {
		maxOffset = 0
	}
	if state.offset > maxOffset {
		state.offset = maxOffset
	}
	if state.offset < 0 {
		state.offset = 0
	}
}

func terminalListRows() int {
	height := terminalHeight()
	rows := height - listHeaderLines
	if rows < 1 {
		return 1
	}
	return rows
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func formatMS(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.0f ms", value)
}

func formatSpeed(value float64) string {
	if value <= 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f", value)
}


