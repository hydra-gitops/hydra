package tui

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// RefRelationLabel returns a short human-readable relation string for a ref edge.
func RefRelationLabel(ref types.Ref) string {
	if len(ref.Labels) > 0 {
		return strings.Join(ref.Labels, ", ")
	}
	return ref.Desc
}

// RefEdgeRow is one selectable row in an incoming or outgoing list.
type RefEdgeRow struct {
	OtherID  types.Id
	Relation string
	Distance int
	IsSelf   bool
	Ref      types.Ref // merged graph edge (labels, tags, attributes, reverse, …)
}

// EdgesForID splits refs into incoming (edges pointing at id) and outgoing (edges from id).
//
// For refs with Reverse=true (for example Kubernetes ownerReferences), Hydra keeps From/To as
// stored (child -> owner) but consumers treat the logical dependency as reversed. The inspect TUI
// mirrors that: when viewing the child, the owner appears under Incoming (logical “from owner”);
// when viewing the owner, the child appears under Outgoing.
func EdgesForID(refs []types.Ref, id types.Id) (incoming, outgoing []RefEdgeRow) {
	for _, r := range refs {
		if r.Reverse {
			if r.From == id {
				incoming = append(incoming, RefEdgeRow{OtherID: r.To, Relation: RefRelationLabel(r), Ref: r})
			} else if r.To == id {
				outgoing = append(outgoing, RefEdgeRow{OtherID: r.From, Relation: RefRelationLabel(r), Ref: r})
			}
			continue
		}
		if r.To == id {
			incoming = append(incoming, RefEdgeRow{OtherID: r.From, Relation: RefRelationLabel(r), Ref: r})
		}
		if r.From == id {
			outgoing = append(outgoing, RefEdgeRow{OtherID: r.To, Relation: RefRelationLabel(r), Ref: r})
		}
	}
	return incoming, outgoing
}

// TransitiveEdgesForID splits refs into transitive incoming and outgoing reachability rows.
func TransitiveEdgesForID(refs []types.Ref, id types.Id) (incoming, outgoing []RefEdgeRow) {
	incomingDetails, outgoingDetails := commands.TransitiveRefDetailsForID(refs, id)
	for _, detail := range incomingDetails {
		incoming = append(incoming, RefEdgeRow{
			OtherID:  detail.ID,
			Relation: RefRelationLabel(detail.Ref),
			Distance: detail.Distance,
			Ref:      detail.Ref,
		})
	}
	for _, detail := range outgoingDetails {
		outgoing = append(outgoing, RefEdgeRow{
			OtherID:  detail.ID,
			Relation: RefRelationLabel(detail.Ref),
			Distance: detail.Distance,
			Ref:      detail.Ref,
		})
	}
	return incoming, outgoing
}

var (
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	panelStyle       = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	listPanelStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderTop(true).BorderLeft(true).BorderRight(true).BorderBottom(false).Padding(0, 1)
	detailPanelStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderTop(false).BorderLeft(true).BorderRight(true).BorderBottom(true).Padding(0, 1)
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11"))
	selStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("14")).Bold(true)
	normalStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("7"))
	helpStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

func panelDivider(contentW int) string {
	return "├" + strings.Repeat("─", contentW+2) + "┤"
}

// RefTreeModel is the Bubble Tea model for hydra (local|cluster) tree.
type RefTreeModel struct {
	allRefs          []types.Ref
	current          types.Id
	history          []types.Id
	allIncoming      []RefEdgeRow
	allOutgoing      []RefEdgeRow
	incoming         []RefEdgeRow
	outgoing         []RefEdgeRow
	filterField      FilterField
	filterQuery      string
	filterOpen       bool
	filterFocus      filterPopupFocus
	filterDraftField FilterField
	filterDraftQuery string
	sortField        refTreeSortField
	sortDescending   bool
	// cursor is a flat index over incoming rows first, then outgoing rows (0..n-1).
	cursor int
	// listScroll is the first visible line index inside refTreeStyledLines (0 = top of list block).
	listScroll int
	// detailScroll is the first visible line index inside refDetailStyledLines for the current selection.
	detailScroll int
	width        int
	height       int
	showStatus   bool
	// allowEscBackToPicker: when true, Esc at the root entity (empty history) exits with exitToPicker
	// so the caller can show the id picker again (hydra local|cluster inspect without <id> on the CLI).
	allowEscBackToPicker bool
	exitToPicker         bool
}

type refTreeLayout struct {
	contentW         int
	listInnerLines   int
	detailInnerLines int
	detailOverflow   bool
}

// NewRefTreeModel builds a model; it recomputes edge lists for the start id.
// allowEscBackToPicker controls Esc at the root entity (see RunRefTree).
func NewRefTreeModel(refs []types.Ref, start types.Id, allowEscBackToPicker bool, showStatus bool) *RefTreeModel {
	m := &RefTreeModel{
		allRefs:              refs,
		current:              start,
		allowEscBackToPicker: allowEscBackToPicker,
		showStatus:           showStatus,
		width:                80,
		height:               24,
		filterField:          FilterFieldID,
		filterDraftField:     FilterFieldID,
		sortField:            refTreeSortDist,
	}
	m.refreshEdges()
	return m
}

const (
	refStatusOK             = "ok"
	refStatusTemplateOnly   = "template only"
	refStatusClusterOnly    = "cluster only"
	refStatusNeither        = "neither"
	refStatusColumnHeader   = "Status"
	refStatusColumnWidth    = 16
	refDistanceColumnHeader = "Dist"
	refDistanceColumnWidth  = 4
	refTreeRowPrefixWidth   = 4
)

// clusterRefEdgeStatus classifies a merged ref edge for cluster inspect display.
// It uses origin:source attributes for template vs cluster provenance.
func clusterRefEdgeStatus(r types.Ref) string {
	var hasTemplate, hasCluster bool
	for _, a := range r.Attributes {
		if a.Type != types.RefAttributeOriginSource {
			continue
		}
		switch a.Value {
		case types.RefSourceTemplate:
			hasTemplate = true
		case types.RefSourceCluster:
			hasCluster = true
		}
	}
	switch {
	case hasTemplate && hasCluster:
		return refStatusOK
	case hasTemplate && !hasCluster:
		return refStatusTemplateOnly
	case !hasTemplate && hasCluster:
		return refStatusClusterOnly
	default:
		return refStatusNeither
	}
}

func (m *RefTreeModel) totalSelectable() int {
	return len(m.incoming) + len(m.outgoing)
}

func (m *RefTreeModel) refreshEdges() {
	m.allIncoming, m.allOutgoing = TransitiveEdgesForID(m.allRefs, m.current)
	m.allOutgoing = append([]RefEdgeRow{{
		OtherID:  m.current,
		Relation: "(self)",
		Distance: 0,
		IsSelf:   true,
	}}, m.allOutgoing...)
	m.applyVisibleRows()
	m.listScroll = 0
	m.detailScroll = 0
	t := m.totalSelectable()
	if t == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= t {
		m.cursor = t - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *RefTreeModel) applyVisibleRows() {
	m.incoming = filterAndSortRefRows(m.allIncoming, m.filterField, m.filterQuery, m.sortField, m.sortDescending, false)
	m.outgoing = filterAndSortRefRows(m.allOutgoing, m.filterField, m.filterQuery, m.sortField, m.sortDescending, true)
	if t := m.totalSelectable(); t == 0 {
		m.cursor = 0
		m.listScroll = 0
		m.detailScroll = 0
		return
	} else if m.cursor >= t {
		m.cursor = t - 1
	}
	m.clampListScrollBounds()
	m.clampListScrollForCursor()
	m.clampDetailScroll()
}

func filterAndSortRefRows(rows []RefEdgeRow, field FilterField, query string, sortField refTreeSortField, descending bool, keepSelf bool) []RefEdgeRow {
	out := make([]RefEdgeRow, 0, len(rows))
	for _, row := range rows {
		if keepSelf && row.IsSelf {
			out = append(out, row)
			continue
		}
		if filterQueryMatch(refRowFieldValue(row, field), query) {
			out = append(out, row)
		}
	}
	start := 0
	if keepSelf && len(out) > 0 && out[0].IsSelf {
		start = 1
	}
	slices.SortFunc(out[start:], func(a, b RefEdgeRow) int {
		var c int
		switch sortField {
		case refTreeSortDist:
			c = cmp.Compare(abs(a.Distance), abs(b.Distance))
		case refTreeSortRelation:
			c = compareLower(a.Relation, b.Relation)
		case refTreeSortStatus:
			c = compareLower(refRowFieldValue(a, FilterFieldStatus), refRowFieldValue(b, FilterFieldStatus))
		default:
			c = compareLower(string(a.OtherID), string(b.OtherID))
		}
		if c != 0 {
			if descending {
				return -c
			}
			return c
		}
		return compareLower(string(a.OtherID), string(b.OtherID))
	})
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func (m *RefTreeModel) selectedRow() *RefEdgeRow {
	t := m.totalSelectable()
	if t == 0 || m.cursor < 0 || m.cursor >= t {
		return nil
	}
	if m.cursor < len(m.incoming) {
		return &m.incoming[m.cursor]
	}
	return &m.outgoing[m.cursor-len(m.incoming)]
}

func (m *RefTreeModel) selectedRef() *types.Ref {
	row := m.selectedRow()
	if row == nil || row.IsSelf {
		return nil
	}
	return &row.Ref
}

// Init implements tea.Model.
func (m *RefTreeModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *RefTreeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.clampListScrollBounds()
		m.clampListScrollForCursor()
		m.clampDetailScroll()
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
				m.applyVisibleRows()
				return m, nil
			case "tab", "shift+tab":
				m.filterFocus = filterPopupFocus((int(m.filterFocus) + 1) % 2)
				return m, nil
			case "up", "k":
				if m.filterFocus == filterPopupFocusField {
					m.filterDraftField = nextFilterField(refTreeFilterFields, m.filterDraftField, -1)
				}
				return m, nil
			case "down", "j":
				if m.filterFocus == filterPopupFocusField {
					m.filterDraftField = nextFilterField(refTreeFilterFields, m.filterDraftField, 1)
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
			return m, tea.Quit
		case "/":
			m.filterOpen = true
			m.filterDraftField = m.filterField
			m.filterDraftQuery = m.filterQuery
			m.filterFocus = filterPopupFocusQuery
			return m, nil
		case "s":
			if m.sortDescending {
				m.sortDescending = false
				m.sortField = nextRefTreeSortField(m.sortField)
			} else {
				m.sortDescending = true
			}
			m.applyVisibleRows()
			return m, nil
		case "S":
			// Reverse of s: ascending on same column, or previous column descending.
			if m.sortDescending {
				m.sortDescending = false
			} else {
				m.sortDescending = true
				m.sortField = prevRefTreeSortField(m.sortField)
			}
			m.applyVisibleRows()
			return m, nil
		case "esc":
			if len(m.history) == 0 {
				if m.allowEscBackToPicker {
					m.exitToPicker = true
				}
				return m, tea.Quit
			}
			m.current = m.history[len(m.history)-1]
			m.history = m.history[:len(m.history)-1]
			m.cursor = 0
			m.refreshEdges()
			return m, nil
		case "up", "k":
			prev := m.cursor
			if m.cursor > 0 {
				m.cursor--
			}
			if m.cursor != prev {
				m.detailScroll = 0
			}
			m.clampListScrollForCursor()
			return m, nil
		case "down", "j":
			prev := m.cursor
			t := m.totalSelectable()
			if t > 0 && m.cursor < t-1 {
				m.cursor++
			}
			if m.cursor != prev {
				m.detailScroll = 0
			}
			m.clampListScrollForCursor()
			return m, nil
		case "[":
			m.scrollDetail(-1)
			return m, nil
		case "]":
			m.scrollDetail(1)
			return m, nil
		case "pgup":
			m.pageListScroll(-1)
			return m, nil
		case "pgdown":
			m.pageListScroll(1)
			return m, nil
		case "enter":
			t := m.totalSelectable()
			if t == 0 || m.cursor < 0 || m.cursor >= t {
				return m, nil
			}
			var row *RefEdgeRow
			if m.cursor < len(m.incoming) {
				row = &m.incoming[m.cursor]
			} else {
				row = &m.outgoing[m.cursor-len(m.incoming)]
			}
			if row.IsSelf {
				return m, nil
			}
			m.history = append(m.history, m.current)
			m.current = row.OtherID
			m.cursor = 0
			m.refreshEdges()
			return m, nil
		}
	}
	return m, nil
}

// entityHeaderPlain is the top line: entity id plus inventory status for the selected edge.
func (m *RefTreeModel) entityHeaderPlain() string {
	base := "Entity: " + string(m.current)
	if !m.showStatus || m.totalSelectable() == 0 {
		return base
	}
	ref := m.selectedRef()
	if ref == nil {
		return base
	}
	return base + " (" + clusterRefEdgeStatus(*ref) + ")"
}

// View implements tea.Model.
func (m *RefTreeModel) View() string {
	if m.width < 20 {
		m.width = 80
	}
	header := headerStyle.Width(m.width).Render(m.entityHeaderPlain())
	layout := m.computeLayout()

	help := helpStyle.Render(m.refTreeHelpPlainForOverflow(layout.detailOverflow))
	helpLine := lipgloss.NewStyle().Width(m.width).Render(help)

	divLine := panelDivider(layout.contentW)
	detailBody := renderRefDetailPane(m.selectedRef(), layout.contentW, layout.detailInnerLines, m.detailScroll)
	panelW := m.width - detailPanelStyle.GetBorderLeftSize() - detailPanelStyle.GetBorderRightSize()
	detailBox := detailPanelStyle.Width(panelW).Render(detailBody)

	headerH := lipgloss.Height(header)
	helpH := lipgloss.Height(helpLine)
	detailBoxH := lipgloss.Height(detailBox)
	divH := lipgloss.Height(divLine)
	const topBorder = 1
	listInner := m.height - headerH - helpH - topBorder - divH - detailBoxH
	if listInner < layout.listInnerLines {
		listInner = layout.listInnerLines
	}
	if listInner < 1 {
		listInner = 1
	}

	m.clampListScrollBounds()
	m.clampDetailScroll()
	allLines := m.refTreeStyledLines(layout.contentW)
	scrollH := refTreeListScrollableHeight(listInner)
	blank := normalStyle.Render("")
	var bodyLines []string
	if len(allLines) == 0 {
		bodyLines = []string{blank}
		for len(bodyLines) < listInner {
			bodyLines = append(bodyLines, blank)
		}
	} else {
		headerLine := allLines[0]
		rest := allLines[1:]
		start := m.listScroll
		if start > len(rest) {
			start = 0
			m.listScroll = 0
		}
		end := len(rest)
		if scrollH > 0 && start+scrollH < end {
			end = start + scrollH
		}
		var scrollLines []string
		if start < len(rest) && end > start {
			scrollLines = rest[start:end]
		}
		for len(scrollLines) < scrollH {
			scrollLines = append(scrollLines, blank)
		}
		bodyLines = append([]string{headerLine}, scrollLines...)
		for len(bodyLines) < listInner {
			bodyLines = append(bodyLines, blank)
		}
	}
	body := strings.Join(bodyLines, "\n")

	listBox := listPanelStyle.Width(panelW).Render(body)
	combinedBox := lipgloss.JoinVertical(lipgloss.Left, listBox, divLine, detailBox)

	contentParts := []string{header, helpLine, combinedBox}
	if m.filterOpen {
		contentParts = append(contentParts, "", m.renderFilterPopup())
	}
	return lipgloss.JoinVertical(lipgloss.Left, contentParts...)
}

func (m *RefTreeModel) renderFilterPopup() string {
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

const refTreeChildIndent = "  "

// refTreeListScrollableHeight is the number of list lines below the sticky column header.
func refTreeListScrollableHeight(listInner int) int {
	if listInner <= 1 {
		return 0
	}
	return listInner - 1
}

func (m *RefTreeModel) panelInnerWidth() int {
	inner := m.width - panelStyle.GetHorizontalFrameSize()
	if inner < 1 {
		return 1
	}
	return inner
}

func (m *RefTreeModel) computeLayout() refTreeLayout {
	contentW := m.panelInnerWidth()
	if contentW < 24 {
		contentW = max(1, m.width-2)
	}

	header := headerStyle.Width(m.width).Render(m.entityHeaderPlain())
	headerH := lipgloss.Height(header)
	// Fixed vertical overhead outside the list/detail content:
	//   3 combined panel (top border + divider + bottom border)
	//   + 2 conservative slack for text wrapping inside the detail panel
	const fixedOverhead = 5

	dLog := refDetailLineCount(m.selectedRef(), contentW)

	help := m.refTreeBaseHelpPlain()
	var lay refTreeLayout
	lay.contentW = contentW

	for range 4 {
		helpLine := lipgloss.NewStyle().Width(m.width).Render(helpStyle.Render(help))
		helpH := lipgloss.Height(helpLine)

		R := m.height - headerH - helpH - fixedOverhead
		if R < 2 {
			R = 2
		}

		listInner, detailInner := computeListDetailInnerSplit(R, dLog, 1)
		overflow := dLog > detailInner

		nextHelp := m.refTreeBaseHelpPlain()
		if overflow {
			nextHelp = nextHelp + "  [] detail"
		}
		lay = refTreeLayout{
			contentW:         contentW,
			listInnerLines:   max(1, listInner),
			detailInnerLines: max(1, detailInner),
			detailOverflow:   overflow,
		}
		if nextHelp == help {
			break
		}
		help = nextHelp
	}
	return lay
}

func computeListDetailInnerSplit(R, dLog, minListInner int) (listInner, detailInner int) {
	if minListInner < 1 {
		minListInner = 1
	}
	if R < 2 {
		return 1, 1
	}
	detailInner = min(dLog, R-minListInner)
	if detailInner < 1 {
		detailInner = 1
	}
	listInner = R - detailInner
	if listInner < minListInner {
		listInner = minListInner
		detailInner = R - listInner
	}
	if dLog > detailInner && detailInner < listInner {
		detailInner = listInner
		listInner = R - detailInner
		if listInner < minListInner {
			listInner = minListInner
			detailInner = R - listInner
		}
	}
	return listInner, detailInner
}

// refTreeBaseHelpPlain is the footer without optional detail-scroll hint (used for layout iteration).
func (m *RefTreeModel) refTreeBaseHelpPlain() string {
	if m.allowEscBackToPicker {
		return "↑/↓ move  PgUp/PgDn page  / filter  s/S sort  Enter follow  Esc back or id list  q quit"
	}
	return "↑/↓ move  PgUp/PgDn page  / filter  s/S sort  Enter follow  Esc back  q quit"
}

// refTreeHelpPlainForOverflow returns the footer line for a known overflow flag (avoids re-entering computeLayout from View).
func (m *RefTreeModel) refTreeHelpPlainForOverflow(detailOverflow bool) string {
	if detailOverflow {
		return m.refTreeBaseHelpPlain() + "  [] detail"
	}
	return m.refTreeBaseHelpPlain()
}

// refTreeHelpPlain is the footer line (unstyled) for layout height and help text.
func (m *RefTreeModel) refTreeHelpPlain() string {
	lay := m.computeLayout()
	return m.refTreeHelpPlainForOverflow(lay.detailOverflow)
}

func (m *RefTreeModel) maxDetailScroll(contentW, innerLines int) int {
	styled := refDetailStyledLines(m.selectedRef(), contentW)
	if len(styled) <= 1 {
		return 0
	}
	bodyLines := len(styled) - 1
	visBody := innerLines - 1
	if visBody < 1 {
		return 0
	}
	maxOff := bodyLines - visBody
	if maxOff < 0 {
		return 0
	}
	return maxOff
}

func (m *RefTreeModel) clampDetailScroll() {
	lay := m.computeLayout()
	maxS := m.maxDetailScroll(lay.contentW, lay.detailInnerLines)
	if m.detailScroll > maxS {
		m.detailScroll = maxS
	}
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
}

func (m *RefTreeModel) scrollDetail(delta int) {
	lay := m.computeLayout()
	m.detailScroll += delta
	maxS := m.maxDetailScroll(lay.contentW, lay.detailInnerLines)
	if m.detailScroll < 0 {
		m.detailScroll = 0
	}
	if m.detailScroll > maxS {
		m.detailScroll = maxS
	}
}

// refTreeStyledLines returns one styled line per row for the incoming/outgoing list (scrollable block).
func (m *RefTreeModel) refTreeStyledLines(maxW int) []string {
	rowW := maxW - refTreeRowPrefixWidth
	if rowW < 12 {
		rowW = 12
	}
	layoutW := rowW
	statusW := 0
	if m.showStatus {
		statusW = refStatusColumnWidth
	}
	distW := refDistanceColumnWidth
	layoutW = rowW - statusW - distW - 2
	if layoutW < 24 {
		layoutW = rowW - distW - 1
		statusW = 0
	}
	idW := max(16, layoutW*6/10)
	relW := layoutW - idW - 3
	if relW < 8 {
		relW = 8
	}
	in := m.incoming
	out := m.outgoing
	cur := m.cursor
	total := len(in) + len(out)

	sortIndicator := "▲"
	if m.sortDescending {
		sortIndicator = "▼"
	}
	distLabel := refDistanceColumnHeader
	idLabel := "Ref id"
	relLabel := "Relation"
	statusLabel := refStatusColumnHeader
	// Reserve one rune per column (▲/▼ or space) so headers do not shift when the active column changes.
	di, ii, ri, si := " ", " ", " ", " "
	switch m.sortField {
	case refTreeSortDist:
		di = sortIndicator
	case refTreeSortID:
		ii = sortIndicator
	case refTreeSortRelation:
		ri = sortIndicator
	case refTreeSortStatus:
		si = sortIndicator
	}
	distLabel += di
	idLabel += ii
	relLabel += ri
	dW := distW - 1
	iW := idW - 1
	rW := relW - 1
	var sW int
	if statusW > 0 {
		statusLabel += si
		sW = statusW - 1
	}
	var hdr string
	if statusW > 0 {
		hdr = fmt.Sprintf("%*s %-*s %-*s %-*s", dW, distLabel, iW, idLabel, rW, relLabel, sW, statusLabel)
	} else {
		hdr = fmt.Sprintf("%*s %-*s %-*s", dW, distLabel, iW, idLabel, rW, relLabel)
	}
	lines := []string{
		normalStyle.Render(hdr),
		titleStyle.Render("Incoming refs"),
	}
	if len(in) == 0 {
		lines = append(lines, normalStyle.Render(refTreeChildIndent+"(none)"))
	} else {
		for i := range in {
			sel := total > 0 && cur == i
			lines = append(lines, renderRefTreeRow(in[i], idW, relW, distW, statusW, sel))
		}
	}
	lines = append(lines, "")
	lines = append(lines, titleStyle.Render("Outgoing refs"))
	if len(out) == 0 {
		lines = append(lines, normalStyle.Render(refTreeChildIndent+"(none)"))
	} else {
		off := len(in)
		for i := range out {
			sel := total > 0 && cur == off+i
			lines = append(lines, renderRefTreeRow(out[i], idW, relW, distW, statusW, sel))
		}
	}
	return lines
}

func (m *RefTreeModel) listInnerMaxLines(contentW int) int {
	return m.computeLayout().listInnerLines
}

func (m *RefTreeModel) cursorLineIndex() int {
	if m.totalSelectable() == 0 {
		return 0
	}
	inRows := len(m.incoming)
	if inRows == 0 {
		inRows = 1
	}
	if m.cursor < len(m.incoming) {
		return 2 + m.cursor
	}
	off := m.cursor - len(m.incoming)
	return 2 + inRows + 2 + off
}

func (m *RefTreeModel) clampListScrollBounds() {
	contentW := m.panelInnerWidth()
	if contentW < 24 {
		contentW = max(1, m.width-2)
	}
	all := m.refTreeStyledLines(contentW)
	maxLines := m.listInnerMaxLines(contentW)
	scrollH := refTreeListScrollableHeight(maxLines)
	if len(all) <= 1 {
		m.listScroll = 0
		return
	}
	rest := len(all) - 1
	maxScroll := rest - scrollH
	if maxScroll < 0 {
		maxScroll = 0
	}
	if m.listScroll > maxScroll {
		m.listScroll = maxScroll
	}
	if m.listScroll < 0 {
		m.listScroll = 0
	}
}

func (m *RefTreeModel) clampListScrollForCursor() {
	if m.totalSelectable() == 0 {
		m.listScroll = 0
		return
	}
	contentW := m.panelInnerWidth()
	if contentW < 24 {
		contentW = max(1, m.width-2)
	}
	all := m.refTreeStyledLines(contentW)
	if len(all) <= 1 {
		m.listScroll = 0
		return
	}
	maxLines := m.listInnerMaxLines(contentW)
	scrollH := refTreeListScrollableHeight(maxLines)
	curLine := m.cursorLineIndex()
	curRest := curLine - 1 // index into lines below the sticky header
	if curRest < 0 {
		curRest = 0
	}
	if curRest < m.listScroll {
		m.listScroll = curRest
	}
	if scrollH > 0 && curRest >= m.listScroll+scrollH {
		m.listScroll = curRest - scrollH + 1
	}
	m.clampListScrollBounds()
}

func (m *RefTreeModel) pageListScroll(dir int) {
	t := m.totalSelectable()
	if t == 0 {
		return
	}
	contentW := m.panelInnerWidth()
	if contentW < 24 {
		contentW = max(1, m.width-2)
	}
	maxL := m.listInnerMaxLines(contentW)
	delta := refTreeListScrollableHeight(maxL)
	if delta < 1 {
		delta = 1
	}
	prev := m.cursor
	if dir < 0 {
		m.cursor -= delta
		if m.cursor < 0 {
			m.cursor = 0
		}
	} else {
		m.cursor += delta
		if m.cursor >= t {
			m.cursor = t - 1
		}
	}
	if m.cursor != prev {
		m.detailScroll = 0
	}
	m.clampListScrollForCursor()
}

func renderRefTreeRow(row RefEdgeRow, idW, relW, distW, statusW int, selected bool) string {
	idStr := truncate(string(row.OtherID), idW)
	relStr := truncate(row.Relation, relW)
	distStr := formatRefDistance(row.Distance)
	var line string
	if statusW > 0 {
		st := "—"
		if !row.IsSelf {
			st = truncate(clusterRefEdgeStatus(row.Ref), statusW)
		}
		line = fmt.Sprintf("%*s %-*s %-*s %-*s", distW, distStr, idW, idStr, relW, relStr, statusW, st)
	} else {
		line = fmt.Sprintf("%*s %-*s %-*s", distW, distStr, idW, idStr, relW, relStr)
	}
	if selected {
		return selStyle.Render(refTreeChildIndent + "▸ " + line)
	}
	return normalStyle.Render(refTreeChildIndent + "  " + line)
}

func formatRefDistance(distance int) string {
	return fmt.Sprintf("%d", distance)
}

func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 3 {
		return string(r[:max])
	}
	return string(r[:max-3]) + "..."
}

// refDetailPlainContentLines returns body lines under the "Selected relation" title (no ANSI).
func refDetailPlainContentLines(ref *types.Ref, maxW int) []string {
	if ref == nil {
		return nil
	}
	r := *ref
	lines := []string{
		"From:    " + truncate(string(r.From), max(8, maxW-10)),
		"To:      " + truncate(string(r.To), max(8, maxW-10)),
		fmt.Sprintf("Reverse: %v", r.Reverse),
		fmt.Sprintf("RefType: %s", r.RefType),
	}
	if len(r.Labels) > 0 {
		lines = append(lines, "Labels:  "+truncate(strings.Join(r.Labels, ", "), max(8, maxW-10)))
	}
	if len(r.Tags) > 0 {
		tags := append([]string{}, r.Tags...)
		slices.Sort(tags)
		lines = append(lines, "Tags:    "+truncate(strings.Join(tags, ", "), max(8, maxW-10)))
	}
	if r.Desc != "" {
		lines = append(lines, "Desc:    "+truncate(r.Desc, max(8, maxW-10)))
	}
	if len(r.Attributes) > 0 {
		attrs := append([]types.RefAttribute{}, r.Attributes...)
		slices.SortFunc(attrs, func(a, b types.RefAttribute) int {
			if c := strings.Compare(a.Type, b.Type); c != 0 {
				return c
			}
			return strings.Compare(a.Value, b.Value)
		})
		lines = append(lines, "Attributes:")
		for _, a := range attrs {
			line := fmt.Sprintf("  %s: %s", a.Type, a.Value)
			lines = append(lines, truncate(line, maxW-2))
		}
	}
	return lines
}

func refDetailLineCount(ref *types.Ref, maxW int) int {
	if ref == nil {
		return 1
	}
	return 1 + len(refDetailPlainContentLines(ref, maxW))
}

// refDetailStyledLines is one styled row per visible line in the detail pane (including title).
func refDetailStyledLines(ref *types.Ref, maxW int) []string {
	if ref == nil {
		return []string{normalStyle.Render("(no row selected)")}
	}
	plain := refDetailPlainContentLines(ref, maxW)
	out := make([]string, 0, 1+len(plain))
	out = append(out, titleStyle.Render("Selected relation"))
	for _, ln := range plain {
		out = append(out, normalStyle.Render(ln))
	}
	return out
}

// renderRefDetailPane renders the bordered detail panel. The title line ("Selected relation") stays
// fixed; only body lines scroll so tests and operators always see a stable anchor in the panel.
func renderRefDetailPane(ref *types.Ref, contentW, innerLines, scrollOff int) string {
	lines := refDetailStyledLines(ref, contentW)
	if innerLines < 1 {
		innerLines = 1
	}
	if len(lines) == 0 {
		return ""
	}
	titleLine := lines[0]
	body := lines[1:]
	bodySlots := innerLines - 1
	if bodySlots < 0 {
		bodySlots = 0
	}
	if scrollOff > len(body) {
		scrollOff = len(body)
	}
	if scrollOff < 0 {
		scrollOff = 0
	}
	end := min(len(body), scrollOff+bodySlots)
	window := append([]string{titleLine}, body[scrollOff:end]...)
	blank := normalStyle.Render("")
	for len(window) < innerLines {
		window = append(window, blank)
	}
	return strings.Join(window, "\n")
}

// renderRefDetail prints the full merged types.Ref for the highlighted list row (labels, tags, attributes).
// Used by tests; caps height with an ellipsis on the last visible line when maxLines is small.
func renderRefDetail(ref *types.Ref, maxW int, maxLines int) string {
	if maxLines < 2 {
		maxLines = 2
	}
	if ref == nil {
		return normalStyle.Render("(no row selected)")
	}
	content := refDetailPlainContentLines(ref, maxW)
	contentBudget := maxLines - 1
	if contentBudget < 1 {
		contentBudget = 1
	}
	if len(content) > contentBudget {
		content = content[:contentBudget]
		content[len(content)-1] = truncate(content[len(content)-1], maxW-4) + " …"
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("Selected relation"))
	b.WriteString("\n")
	for _, line := range content {
		b.WriteString(normalStyle.Render(line))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// RunRefTree starts the full-screen TUI; refs must include every edge needed for navigation.
// allowEscBackToPicker: when true, Esc at the root entity (no navigation history) returns
// backToPicker=true so the caller can show the id list again; when false, Esc always ends the session.
func RunRefTree(refs []types.Ref, start types.Id, allowEscBackToPicker bool, showStatus bool) (backToPicker bool, err error) {
	m := NewRefTreeModel(refs, start, allowEscBackToPicker, showStatus)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}
	rm, ok := finalModel.(*RefTreeModel)
	if !ok || !rm.exitToPicker {
		return false, nil
	}
	return true, nil
}
