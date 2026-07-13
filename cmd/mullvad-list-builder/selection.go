package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

func promptForSelection(rows []selectionRow) ([]int, error) {
	selected, err := runInteractiveSelector(rows)
	if err != nil {
		if errors.Is(err, errSelectionRequiresFallback) {
			return parseFallbackSelection(rows)
		}
		return nil, err
	}
	return selected, nil
}

func parseFallbackSelection(rows []selectionRow) ([]int, error) {
	selected := map[int]struct{}{}
	for i := range rows {
		if i == 0 {
			selected[i] = struct{}{}
		}
	}
	fmt.Println("Interactive mode is unavailable; using simple index input.")
	fmt.Println("Examples: 1,3,5 or 1-3,7 or * . Press enter to keep first row.")
	fmt.Println()
	renderRows(rows, 0, selected)
	fmt.Print("Selection: ")

	in := bufio.NewReader(os.Stdin)
	line, err := in.ReadString('\n')
	if err != nil {
		return nil, err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		if _, ok := selected[0]; ok {
			return []int{0}, nil
		}
		return nil, nil
	}
	if strings.EqualFold(line, "q") {
		return nil, errSelectionCancelled
	}
	return parseSelectionInput(line, len(rows))
}

func runInteractiveSelector(rows []selectionRow) ([]int, error) {
	state := selectorState{
		rows:     rows,
		selected: map[int]struct{}{},
		cursor:   0,
	}

	reader := bufio.NewReaderSize(os.Stdin, 1)
	restore, err := enableRawMode()
	if err != nil {
		return nil, errSelectionRequiresFallback
	}
	defer restore()

	if len(state.rows) == 0 {
		return nil, nil
	}

	for {
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
		case keyAll:
			state.selected = make(map[int]struct{}, len(state.rows))
			for i := range state.rows {
				state.selected[i] = struct{}{}
			}
		case keyClear:
			state.selected = make(map[int]struct{})
		case keyEnter:
			return orderedSelection(state.selected), nil
		}
	}
}

type selectorState struct {
	cursor   int
	rows     []selectionRow
	selected map[int]struct{}
}

func orderedSelection(selected map[int]struct{}) []int {
	out := make([]int, 0, len(selected))
	for idx := range selected {
		out = append(out, idx)
	}
	sort.Ints(out)
	return out
}

func parseSelectionInput(value string, max int) ([]int, error) {
	parts := splitSelection(value)
	if len(parts) == 0 {
		return nil, nil
	}

	set := map[int]struct{}{}
	for _, part := range parts {
		if part == "*" {
			for i := 0; i < max; i++ {
				set[i] = struct{}{}
			}
			continue
		}

		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range: %s", part)
			}
			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start %q: %w", part, err)
			}
			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end %q: %w", part, err)
			}
			if start < 1 || end < 1 || start > max || end > max {
				return nil, fmt.Errorf("selection out of range: %s", part)
			}
			if start > end {
				start, end = end, start
			}
			for i := start - 1; i <= end-1; i++ {
				set[i] = struct{}{}
			}
			continue
		}

		number, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid selection %q: %w", part, err)
		}
		if number < 1 || number > max {
			return nil, fmt.Errorf("selection out of range: %s", part)
		}
		set[number-1] = struct{}{}
	}

	out := orderedSelection(set)
	return out, nil
}

func splitSelection(value string) []string {
	value = strings.TrimSpace(strings.ReplaceAll(value, " ", ""))
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, strings.ToLower(p))
		}
	}
	return out
}

func renderState(state selectorState) {
	fmt.Print("\x1b[2J\x1b[H")
	fmt.Println("Choose providers (city + provider) to add.")
	fmt.Println("  Up/Down move, Space select, A all, C clear, Enter confirm, Q quit")
	fmt.Printf("  Selected %d / %d\n\n", len(state.selected), len(state.rows))
	renderRows(state.rows, state.cursor, state.selected)
}

func renderRows(rows []selectionRow, cursor int, selected map[int]struct{}) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	defer w.Flush()

	_, _ = fmt.Fprintln(w, "Idx\tSel\tCountry\tCity\tProvider\tHost\tStatus\tSpeed\tRelays\tPre ping\tLatency\tDownload\tUpload\tLocation")
	for i, row := range rows {
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
