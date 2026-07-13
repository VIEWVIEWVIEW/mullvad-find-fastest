package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

const listHeaderLines = 6
const (
	colCursorWidth     = 2
	colIdxWidth        = 4
	colSelWidth       = 3
	colCountryWidth   = 8
	colCityWidth      = 6
	colProviderWidth  = 12
	colHostWidth      = 18
	colStatusWidth    = 10
	colSpeedWidth     = 7
	colRelaysWidth    = 6
	colPrePingWidth   = 9
	colLatencyWidth   = 9
	colDownloadWidth  = 9
	colUploadWidth    = 8
	colLocationWidth  = 36
)

var colAlignRight = map[int]bool{
	0:  false, // cursor
	1:  true,  // idx
	2:  false, // sel
	3:  false, // country
	4:  false, // city
	5:  false, // provider
	6:  false, // host
	7:  false, // status
	8:  true,  // speed
	9:  true,  // relays
	10: true,  // pre ping
	11: true,  // latency
	12: true,  // download
	13: true,  // upload
	14: false, // location
}

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

	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	windowRows := maxInt(terminalListRows(), 1)
	prevWindowRows := windowRows
	renderHeader(state)
	prevWindowRows = renderViewport(state, windowRows, prevWindowRows)

	for {
		prevCursor := state.cursor
		prevOffset := state.offset
		prevSelectedCount := len(state.selected)

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
			cleanupSelectionTerminal()
			return orderedSelection(state.selected), nil
		}

		updateOffset(&state)

		windowRows = maxInt(terminalListRows(), 1)
		selectionChanged := len(state.selected) != prevSelectedCount
		cursorMoved := state.cursor != prevCursor
		offsetChanged := state.offset != prevOffset
		headerNeedsUpdate := selectionChanged || cursorMoved || offsetChanged
		viewportChanged := offsetChanged || (windowRows != prevWindowRows)

		if cursorMoved && !viewportChanged {
			// Update only the two rows involved in the cursor move.
			renderRowAt(state, prevCursor, windowRows)
			renderRowAt(state, state.cursor, windowRows)
		} else if viewportChanged || selectionChanged {
			// Redraw visible rows when the viewport changes, or when selection changes but cursor moved.
			prevWindowRows = renderViewport(state, windowRows, prevWindowRows)
		}

		if headerNeedsUpdate {
			renderHeader(state)
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

func renderHeader(state selectorState) {
	windowRows := maxInt(terminalListRows(), 1)
	end := minInt(state.offset+windowRows, len(state.rows))

	writeLineAt(1, "Choose providers (city + provider) to add.")
	writeLineAt(2, "  Use up/down arrows or j/k, Space to select/deselect, Enter to submit, Q to quit")
	writeLineAt(3, fmt.Sprintf("  Selected %d / %d", len(state.selected), len(state.rows)))
	writeLineAt(4, fmt.Sprintf("  Showing rows %d-%d", state.offset+1, end))
	writeLineAt(5, "")
}

func renderViewport(state selectorState, windowRows int, previousRenderedRows int) int {
	dataRows := maxInt(windowRows, 1)
	listHeaderLine := listHeaderLines
	listStartLine := listHeaderLines + 1
	renderedRows := 0

	writeLineAt(listHeaderLine, headerRowLine())

	for i := 0; i < dataRows; i++ {
		rowIndex := state.offset + i
		lineNo := listStartLine + i
		if rowIndex >= len(state.rows) {
			writeLineAt(lineNo, "")
			continue
		}
		writeLineAt(lineNo, formatRowLine(rowIndex, state.rows[rowIndex], state.cursor, state.selected))
		renderedRows++
	}

	if previousRenderedRows > dataRows {
		for i := dataRows; i < previousRenderedRows; i++ {
			writeLineAt(listStartLine+i, "")
		}
	}

	return renderedRows
}

func renderRowAt(state selectorState, rowIndex int, windowRows int) {
	if rowIndex < 0 || rowIndex >= len(state.rows) {
		return
	}
	if rowIndex < state.offset || rowIndex >= state.offset+maxInt(windowRows, 1) {
		return
	}
	lineNo := listHeaderLines + 1 + (rowIndex - state.offset)
	writeLineAt(lineNo, formatRowLine(rowIndex, state.rows[rowIndex], state.cursor, state.selected))
}

func formatRowLine(index int, row selectionRow, cursor int, selected map[int]struct{}) string {
	cursorMark := " "
	if index == cursor {
		cursorMark = ">"
	}
	checkMark := " "
	if _, ok := selected[index]; ok {
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

	return renderTableLine(
		cursorMark,
		strconv.Itoa(index+1),
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

func headerRowLine() string {
	return renderTableLine(" ", "Idx", "Sel", "Country", "City", "Provider", "Host", "Status", "Speed", "Relays", "Pre ping", "Latency", "Download", "Upload", "Location")
}

func renderTableLine(fields ...string) string {
	widths := []int{
		colCursorWidth,
		colIdxWidth,
		colSelWidth,
		colCountryWidth,
		colCityWidth,
		colProviderWidth,
		colHostWidth,
		colStatusWidth,
		colSpeedWidth,
		colRelaysWidth,
		colPrePingWidth,
		colLatencyWidth,
		colDownloadWidth,
		colUploadWidth,
		colLocationWidth,
	}

	var b strings.Builder
	for i, field := range fields {
		width := colCountryWidth
		if i < len(widths) {
			width = widths[i]
		}
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatField(field, width, colAlignRight[i]))
	}
	return b.String()
}

func formatField(value string, width int, alignRight bool) string {
	if width <= 0 {
		return value
	}
	runes := []rune(value)
	if len(runes) > width {
		if width == 1 {
			return value[:1]
		}
		if width > 1 {
			runes = runes[:width-1]
			value = string(runes) + "…"
		}
	}
	if alignRight {
		return fmt.Sprintf("%*s", width, value)
	}
	return fmt.Sprintf("%-*s", width, value)
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

func cleanupSelectionTerminal() {
	fmt.Print("\x1b[0m\x1b[2J\x1b[?25h\x1b[H")
}

func writeLineAt(line int, value string) {
	fmt.Printf("\x1b[%d;1H\x1b[2K%s", line, value)
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
