package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
)

type mouseTarget int

const (
	mouseTargetNone mouseTarget = iota
	mouseTargetTabs
	mouseTargetSection
	mouseTargetPreview
)

// mouseTargetAt returns the part of the screen under the mouse, along with the
// mouse's line relative to the top of the main content.
func (m *Model) mouseTargetAt(mouse tea.Mouse) (mouseTarget, int) {
	tabs := ""
	if m.ctx.View != config.RepoView {
		tabs = m.tabs.View()
	}
	y := mouse.Y - lipgloss.Height(tabs)

	if y < 0 {
		return mouseTargetTabs, y
	}

	if y < m.ctx.MainContentHeight {
		if m.sidebar.IsOpen && m.ctx.PreviewPosition != "bottom" &&
			mouse.X >= m.ctx.MainContentWidth {
			return mouseTargetPreview, y
		}
		return mouseTargetSection, y
	}

	previewBottom := m.ctx.MainContentHeight + m.ctx.Styles.Sidebar.BorderWidth +
		m.ctx.DynamicPreviewHeight
	if m.sidebar.IsOpen && m.ctx.PreviewPosition == "bottom" && y < previewBottom {
		return mouseTargetPreview, y
	}

	return mouseTargetNone, y
}

// isInputActive reports whether something is capturing input, in which case
// mouse events shouldn't change the selection.
func (m *Model) isInputActive() bool {
	currSection := m.getCurrSection()
	if currSection != nil &&
		(currSection.IsSearchFocused() || currSection.IsPromptConfirmationFocused()) {
		return true
	}

	return m.prView.IsTextInputBoxFocused() ||
		m.issueSidebar.IsTextInputBoxFocused() ||
		m.footer.ShowConfirmQuit ||
		m.notificationView.HasPendingAction()
}

func (m *Model) onMouseClick(msg tea.MouseClickMsg) tea.Cmd {
	if m.isInputActive() {
		return nil
	}

	target, y := m.mouseTargetAt(msg.Mouse())
	switch target {
	case mouseTargetTabs:
		id := m.tabs.SectionAt(msg)
		if id < 0 || id == m.currSectionId || m.getSectionAt(id) == nil {
			return nil
		}
		m.setCurrSectionId(id)
		return m.onViewedRowChanged()

	case mouseTargetSection:
		currSection := m.getCurrSection()
		if currSection == nil {
			return nil
		}
		row := currSection.RowAt(y)
		if row < 0 || row == currSection.CurrRow() {
			return nil
		}
		currSection.SelectRow(row)
		return m.onViewedRowChanged()

	case mouseTargetPreview:
		tab := m.prView.TabAt(msg)
		if tab < 0 {
			return nil
		}
		m.prView.GoToTab(tab)
		return m.syncSidebar()
	}

	return nil
}

func (m *Model) onMouseWheel(msg tea.MouseWheelMsg) tea.Cmd {
	target, _ := m.mouseTargetAt(msg.Mouse())
	switch target {
	case mouseTargetPreview:
		switch msg.Button {
		case tea.MouseWheelUp:
			m.sidebar.ScrollUp()
		case tea.MouseWheelDown:
			m.sidebar.ScrollDown()
		}

	case mouseTargetSection:
		currSection := m.getCurrSection()
		if currSection == nil || m.isInputActive() {
			return nil
		}

		var cmds []tea.Cmd
		prevRow := currSection.CurrRow()
		switch msg.Button {
		case tea.MouseWheelUp:
			currSection.PrevRow()
		case tea.MouseWheelDown:
			nextRow := currSection.NextRow()
			if prevRow != nextRow && nextRow == currSection.NumRows()-1 &&
				m.ctx.View != config.RepoView {
				cmds = append(cmds, currSection.FetchNextPageSectionRows()...)
			}
		}

		if currSection.CurrRow() != prevRow {
			cmds = append(cmds, m.onViewedRowChanged())
		}
		return tea.Batch(cmds...)
	}

	return nil
}
