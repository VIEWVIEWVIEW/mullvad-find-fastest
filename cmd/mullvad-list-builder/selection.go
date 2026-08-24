package main

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

const listHeaderLines = 7
const (
	colCursorWidth   = 2
	colIdxWidth      = 4
	colSelWidth      = 3
	colCountryWidth  = 8
	colCityWidth     = 6
	colProviderWidth = 12
	colHostWidth     = 18
	colStatusWidth   = 10
	colSpeedWidth    = 7
	colRelaysWidth   = 6
	colPrePingWidth  = 9
	colLatencyWidth  = 9
	colDownloadWidth = 9
	colUploadWidth   = 8
	colLocationWidth = 36
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

func promptForSelection(rows []selectionRow, filters selectorFilters) ([]int, error) {
	selected, err := runInteractiveSelector(rows, filters)
	return selected, err
}

func runInteractiveSelector(rows []selectionRow, filters selectorFilters) ([]int, error) {
	state := selectorState{
		allRows:  rows,
		selected: map[string]struct{}{},
		cursor:   0,
		offset:   0,
		filters:  filters,
	}
	state.refreshVisible()

	reader := bufio.NewReaderSize(os.Stdin, 1)
	restore, err := enableRawMode()
	if err != nil {
		return nil, err
	}
	defer restore()

	if len(state.visible) == 0 && !state.filters.active() {
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
			if state.cursor < len(state.visible)-1 {
				state.cursor++
			}
		case keyHome:
			state.cursor = 0
		case keyEnd:
			if len(state.visible) > 0 {
				state.cursor = len(state.visible) - 1
			}
		case keySpace:
			if row := state.currentRow(); row != nil {
				key := rowSelectionKey(*row)
				if _, ok := state.selected[key]; ok {
					delete(state.selected, key)
				} else {
					state.selected[key] = struct{}{}
				}
			}
		case keySelectAll:
			state.selectVisible()
		case keyFilter:
			restore()
			fmt.Print("\x1b[?25h")
			filters, err := promptSelectorFilters(state.filters)
			if err != nil {
				return nil, err
			}
			state.filters = filters
			state.refreshVisible()
			state.cursor = 0
			state.offset = 0
			restore, err = enableRawMode()
			if err != nil {
				return nil, err
			}
			fmt.Print("\x1b[?25l")
			fmt.Print("\x1b[0m\x1b[2J\x1b[H")
			windowRows = maxInt(terminalListRows(), 1)
			prevWindowRows = renderViewport(state, windowRows, 0)
			renderHeader(state)
			continue
		case keyEnter:
			cleanupSelectionTerminal()
			return orderedSelection(state.allRows, state.selected), nil
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
	allRows  []selectionRow
	visible  []int
	selected map[string]struct{}
	offset   int
	filters  selectorFilters
}

func (s *selectorState) refreshVisible() {
	s.visible = s.visible[:0]
	for idx, row := range s.allRows {
		if s.filters.matches(row) {
			s.visible = append(s.visible, idx)
		}
	}
	if len(s.visible) == 0 {
		s.cursor = 0
		return
	}
	if s.cursor >= len(s.visible) {
		s.cursor = len(s.visible) - 1
	}
}

func (s selectorState) currentRow() *selectionRow {
	if s.cursor < 0 || s.cursor >= len(s.visible) {
		return nil
	}
	return &s.allRows[s.visible[s.cursor]]
}

func (s *selectorState) selectVisible() {
	for _, idx := range s.visible {
		s.selected[rowSelectionKey(s.allRows[idx])] = struct{}{}
	}
}

func rowSelectionKey(row selectionRow) string {
	return fmt.Sprintf("%s/%s/%d", row.countryCode, row.cityCode, row.provider)
}

func orderedSelection(rows []selectionRow, selected map[string]struct{}) []int {
	out := make([]int, 0, len(selected))
	for idx, row := range rows {
		if _, ok := selected[rowSelectionKey(row)]; ok {
			out = append(out, idx)
		}
	}
	return out
}

func renderHeader(state selectorState) {
	windowRows := maxInt(terminalListRows(), 1)
	end := minInt(state.offset+windowRows, len(state.visible))

	writeLineAt(1, "Choose providers (city + provider) to add.")
	writeLineAt(2, "  Up/down or j/k move | Space select | a select all | f filters | Enter apply | Q quit")
	writeLineAt(3, fmt.Sprintf("  Filters: %s", formatSelectorFilters(state.filters)))
	writeLineAt(4, fmt.Sprintf("  Selected %d | Showing rows %d-%d of %d", len(state.selected), showingStart(state.offset, end), end, len(state.visible)))
	writeLineAt(5, "")
	writeLineAt(6, "")
}

func showingStart(offset, end int) int {
	if end == 0 {
		return 0
	}
	return offset + 1
}

func formatSelectorFilters(filters selectorFilters) string {
	parts := make([]string, 0, 3)
	if filters.maxLatencyMS > 0 {
		parts = append(parts, fmt.Sprintf("latency <= %.0f ms", filters.maxLatencyMS))
	}
	if filters.minDownloadMB > 0 {
		parts = append(parts, fmt.Sprintf("download >= %.1f Mbps", filters.minDownloadMB))
	}
	if filters.minUploadMB > 0 {
		parts = append(parts, fmt.Sprintf("upload >= %.1f Mbps", filters.minUploadMB))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}

func promptSelectorFilters(current selectorFilters) (selectorFilters, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("Interactive filters (blank keeps the current value; '-' clears it)")

	maxLatency, err := promptThreshold(reader, "Max latency (ms)", current.maxLatencyMS)
	if err != nil {
		return selectorFilters{}, err
	}
	minDownload, err := promptThreshold(reader, "Min download (Mbps)", current.minDownloadMB)
	if err != nil {
		return selectorFilters{}, err
	}
	minUpload, err := promptThreshold(reader, "Min upload (Mbps)", current.minUploadMB)
	if err != nil {
		return selectorFilters{}, err
	}
	ownership, err := promptOwnership(reader, current.ownership)
	if err != nil {
		return selectorFilters{}, err
	}

	return selectorFilters{
		maxLatencyMS:  maxLatency,
		minDownloadMB: minDownload,
		minUploadMB:   minUpload,
		ownership:     ownership,
	}, nil
}

func promptOwnership(reader *bufio.Reader, current string) (string, error) {
	if current == "" {
		current = "all"
	}
	fmt.Printf("Provider ownership (all/rented/owned) [%s]: ", current)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	if value == "-" {
		return "all", nil
	}
	normalized := normalizeOwnershipFilter(value)
	if normalized == "" {
		return "", fmt.Errorf("invalid provider ownership %q; use all, rented, or owned", value)
	}
	return normalized, nil
}

func promptThreshold(reader *bufio.Reader, label string, current float64) (float64, error) {
	currentText := "none"
	if current > 0 {
		currentText = strconv.FormatFloat(current, 'f', -1, 64)
	}
	fmt.Printf("%s [%s]: ", label, currentText)
	line, err := reader.ReadString('\n')
	if err != nil {
		return 0, err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	if value == "-" || strings.EqualFold(value, "none") {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
		if err == nil {
			err = fmt.Errorf("threshold must be a finite non-negative number")
		}
		return 0, fmt.Errorf("invalid %s: %w", label, err)
	}
	return parsed, nil
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
		if rowIndex >= len(state.visible) {
			writeLineAt(lineNo, "")
			continue
		}
		allIndex := state.visible[rowIndex]
		writeLineAt(lineNo, formatRowLine(rowIndex, state.allRows[allIndex], state.cursor, state.selected))
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
	if rowIndex < 0 || rowIndex >= len(state.visible) {
		return
	}
	if rowIndex < state.offset || rowIndex >= state.offset+maxInt(windowRows, 1) {
		return
	}
	lineNo := listHeaderLines + 1 + (rowIndex - state.offset)
	allIndex := state.visible[rowIndex]
	writeLineAt(lineNo, formatRowLine(rowIndex, state.allRows[allIndex], state.cursor, state.selected))
}

func formatRowLine(index int, row selectionRow, cursor int, selected map[string]struct{}) string {
	cursorMark := " "
	if index == cursor {
		cursorMark = ">"
	}
	checkMark := " "
	if _, ok := selected[rowSelectionKey(row)]; ok {
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
	if len(state.visible) == 0 {
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
	maxOffset := len(state.visible) - windowRows
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
