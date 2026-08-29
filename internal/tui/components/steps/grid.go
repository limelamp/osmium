package steps

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/limelamp/osmium/internal/tui/core"
	"github.com/limelamp/osmium/internal/tui/styles"
	"github.com/limelamp/osmium/internal/tui/theme"
)

// gridGutter is the blank space kept between two columns.
const gridGutter = 3

// minColumnWidth keeps narrow columns readable and the layout even.
const minColumnWidth = 14

// GridColumn is one titled column of choices.
type GridColumn struct {
	Title string
	Items []string
}

// Grid is the column-of-lists selector shared by the engine and version steps:
// choices are grouped into titled columns, ←/→ moves between columns and ↑/↓
// within one. When more columns exist than fit, the grid scrolls sideways.
type Grid struct {
	columns []GridColumn

	col    int
	row    int
	offset int // index of the leftmost visible column
	top    int // index of the first visible item inside a column

	layout core.Layout
}

// NewGrid builds a grid over the given columns.
func NewGrid(columns []GridColumn) Grid {
	return Grid{columns: columns}
}

// Columns reports the grid's contents.
func (g Grid) Columns() []GridColumn { return g.columns }

// Selected returns the column title and item under the cursor.
func (g Grid) Selected() (column, item string) {
	if len(g.columns) == 0 {
		return "", ""
	}

	col := g.columns[g.col]
	if g.row >= len(col.Items) {
		return col.Title, ""
	}
	return col.Title, col.Items[g.row]
}

// SetLayout records the rectangle the grid draws in, which is also what turns
// mouse coordinates into cells.
func (g Grid) SetLayout(l core.Layout) Grid {
	g.layout = l
	return g.follow()
}

// visibleRows is how many items of a column fit, leaving room for the column
// title, its blank line and the range label underneath.
func (g Grid) visibleRows() int {
	return max(1, g.layout.Height-4)
}

// tallestColumn is the item count of the longest visible column.
func (g Grid) tallestColumn() int {
	tallest := 0
	for _, column := range g.columns {
		tallest = max(tallest, len(column.Items))
	}
	return tallest
}

// columnWidth is how wide every column is drawn, sized to the longest item.
func (g Grid) columnWidth() int {
	width := minColumnWidth
	for _, column := range g.columns {
		width = max(width, lipgloss.Width(column.Title))
		for _, item := range column.Items {
			width = max(width, lipgloss.Width(item)+2) // room for the cursor marker
		}
	}
	return width + gridGutter
}

// visibleColumns is how many whole columns fit in the current width.
func (g Grid) visibleColumns() int {
	return max(1, g.layout.Width/g.columnWidth())
}

// follow scrolls sideways so the selected column stays on screen.
func (g Grid) follow() Grid {
	visible := g.visibleColumns()
	if g.col < g.offset {
		g.offset = g.col
	} else if g.col >= g.offset+visible {
		g.offset = g.col - visible + 1
	}
	g.offset = max(0, min(g.offset, max(0, len(g.columns)-visible)))

	// Columns can be longer than the body, so they scroll vertically too.
	rows := g.visibleRows()
	if g.row < g.top {
		g.top = g.row
	} else if g.row >= g.top+rows {
		g.top = g.row - rows + 1
	}
	g.top = max(0, min(g.top, max(0, g.tallestColumn()-rows)))
	return g
}

// moveColumn steps sideways, wrapping between the first and last column and
// keeping the row within the column it lands in.
func (g Grid) moveColumn(delta int) Grid {
	count := len(g.columns)
	if count == 0 {
		return g
	}

	g.col = ((g.col+delta)%count + count) % count
	g.row = min(g.row, max(0, len(g.columns[g.col].Items)-1))
	return g.follow()
}

// moveRow steps up or down inside the current column, wrapping around its ends
// the way the lists do.
func (g Grid) moveRow(delta int) Grid {
	if len(g.columns) == 0 {
		return g
	}

	items := len(g.columns[g.col].Items)
	if items == 0 {
		return g
	}

	g.row = ((g.row+delta)%items + items) % items
	return g.follow()
}

// cellAt maps an absolute terminal coordinate to a grid cell.
func (g Grid) cellAt(x, y int) (col, row int, ok bool) {
	if !g.layout.Hit(x, y) {
		return 0, 0, false
	}

	// Two rows of chrome sit above the items: the column title and a blank.
	row = g.top + (y - g.layout.Y - 2)
	col = g.offset + (x-g.layout.X)/g.columnWidth()

	if row < 0 || col < 0 || col >= len(g.columns) || row >= len(g.columns[col].Items) {
		return 0, 0, false
	}
	return col, row, true
}

// Scroll reports how far down a column the grid is looking.
func (g Grid) Scroll() core.Scroll {
	return core.Scroll{Offset: g.top, Total: g.tallestColumn(), Visible: g.visibleRows()}
}

// Update applies navigation input. It reports whether the input activated the
// selected cell, which the owning step treats like pressing enter.
func (g Grid) Update(msg tea.Msg) (Grid, bool) {
	switch msg := msg.(type) {
	case tea.MouseMsg:
		mouse := msg.Mouse()

		switch msg.(type) {
		case tea.MouseWheelMsg:
			if !g.layout.Hit(mouse.X, mouse.Y) {
				return g, false
			}
			switch mouse.Button {
			case tea.MouseWheelUp:
				g.top = max(0, g.top-core.WheelStep)
			case tea.MouseWheelDown:
				g.top = min(g.top+core.WheelStep, max(0, g.tallestColumn()-g.visibleRows()))
			}
			return g, false

		case tea.MouseClickMsg:
			if mouse.Button != tea.MouseLeft {
				return g, false
			}
			col, row, ok := g.cellAt(mouse.X, mouse.Y)
			if !ok {
				return g, false
			}
			// Clicking the selected cell again confirms it.
			if col == g.col && row == g.row {
				return g, true
			}
			g.col, g.row = col, row
			return g.follow(), false
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "left", "h":
			return g.moveColumn(-1), false
		case "right", "l":
			return g.moveColumn(1), false
		case "up", "k":
			return g.moveRow(-1), false
		case "down", "j":
			return g.moveRow(1), false
		case "enter", "space":
			return g, true
		}
	}

	return g, false
}

// View renders the visible columns side by side.
func (g Grid) View() string {
	if len(g.columns) == 0 {
		return lipgloss.NewStyle().Foreground(theme.Inactive).Render("Nothing to choose from.")
	}

	width := g.columnWidth()
	visible := g.visibleColumns()
	end := min(len(g.columns), g.offset+visible)

	rendered := make([]string, 0, end-g.offset)
	for i := g.offset; i < end; i++ {
		rendered = append(rendered, g.renderColumn(i, width))
	}

	view := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if label := g.rangeLabel(end, visible); label != "" {
		view += "\n\n" + lipgloss.NewStyle().Foreground(theme.Inactive).Render(label)
	}
	return view
}

// renderColumn draws one column: its title, then its items.
func (g Grid) renderColumn(index, width int) string {
	column := g.columns[index]

	title := lipgloss.NewStyle().Foreground(theme.Inactive).Render(column.Title)
	if index == g.col {
		title = lipgloss.NewStyle().Foreground(theme.Accent).Bold(true).Render(column.Title)
	}

	rows := []string{title, ""}
	end := min(len(column.Items), g.top+g.visibleRows())
	for i := g.top; i < end; i++ {
		rows = append(rows, styles.ListRow(width-gridGutter, index == g.col && i == g.row, column.Items[i], ""))
	}

	return lipgloss.NewStyle().Width(width).Render(lipgloss.JoinVertical(lipgloss.Left, rows...))
}

// rangeLabel tells the user what is off screen, horizontally or vertically. It
// is empty when everything already fits.
func (g Grid) rangeLabel(end, visible int) string {
	var parts []string

	if len(g.columns) > visible {
		parts = append(parts, "showing "+g.columns[g.offset].Title+" – "+g.columns[end-1].Title+"  ·  [←/→] more columns")
	}
	if g.tallestColumn() > g.visibleRows() {
		parts = append(parts, "[↑/↓] more versions")
	}

	return strings.Join(parts, "   •   ")
}
