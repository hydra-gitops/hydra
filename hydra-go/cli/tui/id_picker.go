package tui

import (
	"fmt"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// idPickerStatusColWidth matches ref inspect Status column width (longest: "template only").
const idPickerStatusColWidth = 16

// IDPickerModel lists resource ids with a popup-based field filter and cycling sort modes.
type IDPickerModel struct {
	title    string
	allIds   []types.Id
	filtered []types.Id
	// statusByID maps each id to ok | template only | cluster only | neither (nil = no Status column).
	statusByID map[types.Id]string

	filterField      FilterField
	filterQuery      string
	filterOpen       bool
	filterDraftField FilterField
	filterDraftQuery string
	filterFocus      filterPopupFocus
	sortField        pickerSortField
	sortDescending   bool

	cursor int
	scroll int

	width  int
	height int

	selected  types.Id
	cancelled bool
}

// NewIDPickerModel builds a picker over the given ids (should be pre-sorted for stable UX).
// statusByID may be nil; when set, a Status column is shown (same labels as hydra gitops inspect).
func NewIDPickerModel(title string, ids []types.Id, statusByID map[types.Id]string) *IDPickerModel {
	m := &IDPickerModel{
		title:            title,
		allIds:           ids,
		statusByID:       statusByID,
		width:            80,
		height:           24,
		filterField:      FilterFieldID,
		filterDraftField: FilterFieldID,
		sortField:        pickerSortID,
	}
	m.applyFilters()
	return m
}

func (m *IDPickerModel) applyFilters() {
	m.filtered = FilterTreeIDs(m.allIds, m.filterField, m.filterQuery)
	sortPickerIDs(m.filtered, m.sortField, m.sortDescending)
	m.cursor = 0
	m.scroll = 0
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.clampScroll()
}

func (m *IDPickerModel) clampScroll() {
	rows := m.listRows()
	if rows < 1 {
		m.scroll = 0
		return
	}
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+rows {
		m.scroll = m.cursor - rows + 1
	}
}

func (m *IDPickerModel) listRows() int {
	avail := m.height - 6
	if m.filterOpen {
		avail -= 7
	}
	if avail < 3 {
		avail = 3
	}
	return avail
}

func (m *IDPickerModel) pageScroll(dir int) {
	rows := m.listRows()
	if rows < 1 {
		return
	}
	if dir < 0 {
		m.scroll -= rows
		if m.scroll < 0 {
			m.scroll = 0
		}
	} else {
		m.scroll += rows
		maxScroll := max(0, len(m.filtered)-rows)
		if m.scroll > maxScroll {
			m.scroll = maxScroll
		}
	}
	m.clampScroll()
}

func (m *IDPickerModel) idColumnMaxWidth() int {
	if m.statusByID == nil {
		return max(20, m.width-4)
	}
	w := m.width - 4 - idPickerStatusColWidth - 1
	if w < 20 {
		w = 20
	}
	return w
}

func (m *IDPickerModel) formatListLine(id types.Id) string {
	maxW := m.idColumnMaxWidth()
	idStr := truncate(string(id), maxW)
	if m.statusByID == nil {
		return idStr
	}
	st := m.statusByID[id]
	if st == "" {
		st = "—"
	}
	st = truncate(st, idPickerStatusColWidth)
	return fmt.Sprintf("%-*s %-*s", maxW, idStr, idPickerStatusColWidth, st)
}

// FilterTreeIDs returns ids whose selected field matches the substring filter (case-insensitive).
func FilterTreeIDs(ids []types.Id, field FilterField, query string) []types.Id {
	out := make([]types.Id, 0, len(ids))
	for _, id := range ids {
		if filterQueryMatch(idFieldValue(id, field), query) {
			out = append(out, id)
		}
	}
	return out
}

func sortPickerIDs(ids []types.Id, field pickerSortField, descending bool) {
	slices.SortFunc(ids, func(a, b types.Id) int {
		var av, bv string
		switch field {
		case pickerSortKind:
			av = idFieldValue(a, FilterFieldKind)
			bv = idFieldValue(b, FilterFieldKind)
		case pickerSortNamespace:
			av = idFieldValue(a, FilterFieldNamespace)
			bv = idFieldValue(b, FilterFieldNamespace)
		case pickerSortName:
			av = idFieldValue(a, FilterFieldName)
			bv = idFieldValue(b, FilterFieldName)
		default:
			av = string(a)
			bv = string(b)
		}
		if c := compareLower(av, bv); c != 0 {
			if descending {
				return -c
			}
			return c
		}
		return compareLower(string(a), string(b))
	})
}

func idGVKString(id types.Id) string {
	g, v, k, _, _, err := id.Components()
	if err != nil {
		return string(id)
	}
	if g == "" {
		return fmt.Sprintf("%s/%s", v, k)
	}
	return fmt.Sprintf("%s/%s/%s", g, v, k)
}

// Init implements tea.Model.
func (m *IDPickerModel) Init() tea.Cmd { return nil }

func (m *IDPickerModel) openFilterPopup() {
	m.filterOpen = true
	m.filterDraftField = m.filterField
	m.filterDraftQuery = m.filterQuery
	m.filterFocus = filterPopupFocusQuery
}

// Update implements tea.Model.
func (m *IDPickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampScroll()
		return m, nil
	case tea.KeyMsg:
		if m.filterOpen {
			switch msg.String() {
			case "esc":
				m.filterOpen = false
				return m, nil
			case "enter":
				m.filterField = m.filterDraftField
				m.filterQuery = m.filterDraftQuery
				m.filterOpen = false
				m.applyFilters()
				return m, nil
			case "tab", "shift+tab":
				m.filterFocus = filterPopupFocus((int(m.filterFocus) + 1) % 2)
				return m, nil
			case "up", "k":
				if m.filterFocus == filterPopupFocusField {
					m.filterDraftField = nextFilterField(pickerFilterFields, m.filterDraftField, -1)
				}
				return m, nil
			case "down", "j":
				if m.filterFocus == filterPopupFocusField {
					m.filterDraftField = nextFilterField(pickerFilterFields, m.filterDraftField, 1)
				}
				return m, nil
			case "backspace":
				if m.filterFocus == filterPopupFocusQuery && len(m.filterDraftQuery) > 0 {
					m.filterDraftQuery = m.filterDraftQuery[:len(m.filterDraftQuery)-1]
				}
				return m, nil
			}
			if msg.Type == tea.KeyRunes && m.filterFocus == filterPopupFocusQuery {
				m.filterDraftQuery += string(msg.Runes)
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			m.cancelled = true
			return m, tea.Quit
		case "esc":
			m.cancelled = true
			return m, tea.Quit
		case "/":
			m.openFilterPopup()
			return m, nil
		case "s":
			if m.sortDescending {
				m.sortDescending = false
				m.sortField = nextPickerSortField(m.sortField)
			} else {
				m.sortDescending = true
			}
			m.applyFilters()
			return m, nil
		case "S":
			if m.sortDescending {
				m.sortDescending = false
			} else {
				m.sortDescending = true
				m.sortField = prevPickerSortField(m.sortField)
			}
			m.applyFilters()
			return m, nil
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.clampScroll()
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.clampScroll()
			}
		case "pgup":
			m.pageScroll(-1)
		case "pgdown":
			m.pageScroll(1)
		case "enter":
			if len(m.filtered) > 0 {
				m.selected = m.filtered[m.cursor]
				return m, tea.Quit
			}
		}
		return m, nil
	}
	return m, nil
}

// View implements tea.Model.
func (m *IDPickerModel) View() string {
	if m.width < 30 {
		m.width = 80
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(m.title))
	b.WriteString("\n\n")

	filterSummary := "off"
	if strings.TrimSpace(m.filterQuery) != "" {
		filterSummary = fmt.Sprintf("%s contains %q", m.filterField, m.filterQuery)
	}
	sortIndicator := "▲"
	if m.sortDescending {
		sortIndicator = "▼"
	}
	b.WriteString(helpStyle.Render(fmt.Sprintf("Sort: %s%s  Filter: %s", m.sortField, sortIndicator, filterSummary)))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("/: filter  s/S: sort  ↑/↓: list  PgUp/PgDn: page  Enter: choose  Esc/q: quit"))
	b.WriteString("\n\n")

	b.WriteString(titleStyle.Render(fmt.Sprintf("IDs (%d / %d)", len(m.filtered), len(m.allIds))))
	b.WriteString("\n")
	if m.statusByID != nil {
		hdr := fmt.Sprintf("%-*s %-*s", m.idColumnMaxWidth(), "Resource id", idPickerStatusColWidth, "Status")
		b.WriteString(normalStyle.Render(hdr))
		b.WriteString("\n")
	}

	rows := m.listRows()
	start := m.scroll
	end := start + rows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}
	if start > 0 && len(m.filtered) > 0 {
		b.WriteString(helpStyle.Render(fmt.Sprintf("… %d above\n", start)))
	}
	for i := start; i < end; i++ {
		line := m.formatListLine(m.filtered[i])
		if i == m.cursor {
			b.WriteString(selStyle.Render("▸ " + line))
		} else {
			b.WriteString(normalStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	if end < len(m.filtered) {
		b.WriteString(helpStyle.Render(fmt.Sprintf("… %d below\n", len(m.filtered)-end)))
	}
	if len(m.filtered) == 0 {
		b.WriteString(normalStyle.Render("(no ids match filters)"))
		b.WriteString("\n")
	}

	if m.filterOpen {
		b.WriteString("\n")
		b.WriteString(m.renderFilterPopup())
	}

	return b.String()
}

func (m *IDPickerModel) renderFilterPopup() string {
	queryLabel := "Query"
	fieldLabel := "Field"
	if m.filterFocus == filterPopupFocusQuery {
		queryLabel = "▶ " + queryLabel
	}
	if m.filterFocus == filterPopupFocusField {
		fieldLabel = "▶ " + fieldLabel
	}
	queryValue := m.filterDraftQuery
	if m.filterFocus == filterPopupFocusQuery {
		queryValue += "▏"
	}
	body := strings.Join([]string{
		titleStyle.Render("Filter"),
		fmt.Sprintf("%s: %s", queryLabel, queryValue),
		fmt.Sprintf("%s: %s", fieldLabel, m.filterDraftField),
		helpStyle.Render("Tab: switch  ↑/↓: field  Enter: apply  Esc: close"),
	}, "\n")
	boxW := min(max(36, m.width-8), 56)
	box := panelStyle.Width(boxW).Render(body)
	margin := max(0, (m.width-lipgloss.Width(box))/2)
	return lipgloss.NewStyle().MarginLeft(margin).Render(box)
}

// RunIDPicker shows the picker and returns the selected id, or empty if the user cancelled.
// statusByID may be nil; when set, a Status column is shown (ok / missing / cluster only).
func RunIDPicker(title string, ids []types.Id, statusByID map[types.Id]string) (types.Id, error) {
	m := NewIDPickerModel(title, ids, statusByID)
	p := tea.NewProgram(m, tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		return "", err
	}
	pm, ok := final.(*IDPickerModel)
	if !ok {
		return "", nil
	}
	if pm.cancelled {
		return "", nil
	}
	return pm.selected, nil
}
