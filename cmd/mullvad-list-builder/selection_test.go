package main

import "testing"

func testSelectionRow(country, city string, provider int, latency, download, upload float64, hasSpeed bool) selectionRow {
	return selectionRow{
		countryCode: country,
		cityCode:    city,
		provider:    provider,
		latencyMS:   latency,
		downloadMB:  download,
		uploadMB:    upload,
		hasSpeed:    hasSpeed,
	}
}

func TestSelectorFilters(t *testing.T) {
	row := testSelectionRow("de", "ber", 1, 40, 200, 30, true)
	row.providerStatus = "mullvad-owned"
	rentedRow := testSelectionRow("nl", "ams", 2, 40, 200, 30, true)
	rentedRow.providerStatus = "rented"
	tests := []struct {
		name    string
		filters selectorFilters
		row     selectionRow
		want    bool
	}{
		{name: "no filters", row: row, want: true},
		{name: "latency at limit", row: row, filters: selectorFilters{maxLatencyMS: 40}, want: true},
		{name: "latency above limit", row: row, filters: selectorFilters{maxLatencyMS: 39}, want: false},
		{name: "download at limit", row: row, filters: selectorFilters{minDownloadMB: 200}, want: true},
		{name: "download below limit", row: row, filters: selectorFilters{minDownloadMB: 201}, want: false},
		{name: "upload below limit", row: row, filters: selectorFilters{minUploadMB: 31}, want: false},
		{name: "owned matches", row: row, filters: selectorFilters{ownership: "owned"}, want: true},
		{name: "owned excludes rented", row: rentedRow, filters: selectorFilters{ownership: "owned"}, want: false},
		{name: "rented matches", row: rentedRow, filters: selectorFilters{ownership: "rented"}, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filters.matches(tt.row); got != tt.want {
				t.Fatalf("matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectorFiltersHideRowsWithoutSpeedWhenActive(t *testing.T) {
	row := testSelectionRow("de", "ber", 1, 0, 0, 0, false)
	if (selectorFilters{maxLatencyMS: 100}).matches(row) {
		t.Fatal("active filters must hide rows without speed measurements")
	}
}

func TestSelectorStateRefreshVisibleAndOrderedSelection(t *testing.T) {
	rows := []selectionRow{
		testSelectionRow("de", "ber", 1, 40, 200, 30, true),
		testSelectionRow("nl", "ams", 2, 80, 150, 20, true),
		testSelectionRow("fr", "par", 3, 120, 100, 15, true),
	}
	state := selectorState{
		allRows:  rows,
		filters:  selectorFilters{maxLatencyMS: 80},
		selected: map[string]struct{}{rowSelectionKey(rows[1]): {}, rowSelectionKey(rows[2]): {}},
	}
	state.refreshVisible()

	if got, want := len(state.visible), 2; got != want {
		t.Fatalf("visible rows = %d, want %d", got, want)
	}
	selected := orderedSelection(rows, state.selected)
	if len(selected) != 2 || selected[0] != 1 || selected[1] != 2 {
		t.Fatalf("ordered selection = %v, want [1 2]", selected)
	}
}

func TestSelectorStateSelectVisible(t *testing.T) {
	rows := []selectionRow{
		testSelectionRow("de", "ber", 1, 40, 200, 30, true),
		testSelectionRow("nl", "ams", 2, 80, 150, 20, true),
		testSelectionRow("fr", "par", 3, 120, 100, 15, true),
	}
	state := selectorState{
		allRows:  rows,
		filters:  selectorFilters{maxLatencyMS: 80},
		selected: map[string]struct{}{rowSelectionKey(rows[2]): {}},
	}
	state.refreshVisible()
	state.selectVisible()

	selected := orderedSelection(rows, state.selected)
	if len(selected) != 3 || selected[0] != 0 || selected[1] != 1 || selected[2] != 2 {
		t.Fatalf("select all visible = %v, want [0 1 2] including prior hidden selection", selected)
	}
}

func TestNormalizeOwnershipFilter(t *testing.T) {
	for input, want := range map[string]string{
		"":              "all",
		"all":           "all",
		"owned":         "owned",
		"mullvad-owned": "owned",
		"rented":        "rented",
		"unknown":       "",
	} {
		if got := normalizeOwnershipFilter(input); got != want {
			t.Errorf("normalizeOwnershipFilter(%q) = %q, want %q", input, got, want)
		}
	}
}
