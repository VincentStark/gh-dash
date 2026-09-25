package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	zone "github.com/lrstanley/bubblezone/v2"
	"github.com/stretchr/testify/require"

	"github.com/dlvhdr/gh-dash/v4/internal/config"
	"github.com/dlvhdr/gh-dash/v4/internal/data"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/branchsidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/footer"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/issueview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/notificationview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prrow"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prssection"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/prview"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/section"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/sidebar"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/components/tabs"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/context"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/keys"
	"github.com/dlvhdr/gh-dash/v4/internal/tui/theme"
)

func newMouseTestModel(t *testing.T, position string) *Model {
	t.Helper()
	zone.NewGlobal()
	zone.SetEnabled(true)
	t.Cleanup(func() { zone.SetEnabled(false) })

	cfg, err := config.ParseConfig(config.Location{
		ConfigFlag:       "../config/testdata/test-config.yml",
		SkipGlobalConfig: true,
	})
	require.NoError(t, err)
	cfg.Defaults.Preview.Width = 0.4
	cfg.Defaults.Preview.Height = 0.4
	cfg.Defaults.Preview.Position = position

	ctx := &context.ProgramContext{
		Config:       &cfg,
		ScreenWidth:  160,
		ScreenHeight: 40,
		View:         config.PRsView,
		StartTask:    func(task context.Task) tea.Cmd { return nil },
	}
	ctx.Theme = theme.ParseTheme(ctx.Config)
	ctx.Styles = context.InitStyles(ctx.Theme)

	var sections []section.Section
	for i, title := range []string{"First", "Second"} {
		s := prssection.NewModel(
			i,
			ctx,
			config.PrsSectionConfig{Title: title, Filters: "is:open"},
			time.Now(),
			time.Now(),
		)
		for j := range 30 {
			s.Prs = append(s.Prs, prrow.Data{
				Primary: &data.PullRequestData{
					Number:     j,
					Title:      fmt.Sprintf("pr-title-%d", j),
					State:      "OPEN",
					Url:        fmt.Sprintf("https://github.com/o/r/pull/%d", j),
					Repository: data.Repository{NameWithOwner: "o/r"},
				},
				IsEnriched: true,
			})
		}
		s.SetIsLoading(false)
		sections = append(sections, &s)
	}

	m := &Model{
		ctx:              ctx,
		keys:             keys.Keys,
		prs:              sections,
		sidebar:          sidebar.NewModel(),
		footer:           footer.NewModel(ctx),
		tabs:             tabs.NewModel(ctx),
		prView:           prview.NewModel(ctx),
		issueSidebar:     issueview.NewModel(ctx),
		branchSidebar:    branchsidebar.NewModel(ctx),
		notificationView: notificationview.NewModel(ctx),
	}
	m.tabs.SetSections(sections)
	m.sidebar.IsOpen = true
	m.syncMainContentDimensions()
	m.syncProgramContext()
	for _, s := range sections {
		s.(*prssection.Model).Table.SetRows(s.BuildRows())
	}
	m.syncSidebar()
	return m
}

// screenPos returns the position of the first occurrence of text on screen,
// looking only at the list when inList is set.
func screenPos(t *testing.T, m *Model, text string, inList bool) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(m.View().Content, "\n") {
		line = ansi.Strip(line)
		if inList && m.ctx.PreviewPosition == "right" {
			line = ansi.Cut(line, 0, m.ctx.MainContentWidth)
		}
		if x := strings.Index(line, text); x >= 0 {
			return ansi.StringWidth(line[:x]), y
		}
	}
	t.Fatalf("%q not found on screen", text)
	return 0, 0
}

func click(m *Model, x, y int) {
	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

func wheel(m *Model, x, y int, button tea.MouseButton) {
	m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: button})
}

func TestMouse_ClickRowSelectsIt(t *testing.T) {
	for _, position := range []string{"right", "bottom"} {
		t.Run(position, func(t *testing.T) {
			m := newMouseTestModel(t, position)

			x, y := screenPos(t, m, "pr-title-4", true)
			click(m, x, y)
			require.Equal(t, 4, m.getCurrSection().CurrRow())

			// Every line of a row selects it.
			_, y = screenPos(t, m, "#2 by", true)
			click(m, x, y)
			require.Equal(t, 2, m.getCurrSection().CurrRow())
			_, y = screenPos(t, m, "pr-title-3", true)
			click(m, x, y)
			require.Equal(t, 3, m.getCurrSection().CurrRow())

			// Clicks on the table header don't change the selection.
			hx, hy := screenPos(t, m, "Title", true)
			click(m, hx, hy)
			require.Equal(t, 3, m.getCurrSection().CurrRow())
		})
	}
}

func TestMouse_WheelOverListMovesSelection(t *testing.T) {
	m := newMouseTestModel(t, "right")
	x, y := screenPos(t, m, "pr-title-0", true)

	for range 3 {
		wheel(m, x, y, tea.MouseWheelDown)
	}
	require.Equal(t, 3, m.getCurrSection().CurrRow())

	wheel(m, x, y, tea.MouseWheelUp)
	require.Equal(t, 2, m.getCurrSection().CurrRow())

	// Scroll well past the first page, then click a row that's now visible.
	for range 20 {
		wheel(m, x, y, tea.MouseWheelDown)
	}
	require.Equal(t, 22, m.getCurrSection().CurrRow())

	// Clicking visible rows selects them without scrolling the list.
	for _, row := range []int{20, 22, 15} {
		_, before := screenPos(t, m, "pr-title-20", true)
		x, y = screenPos(t, m, fmt.Sprintf("pr-title-%d", row), true)
		click(m, x, y)
		require.Equal(t, row, m.getCurrSection().CurrRow())
		_, after := screenPos(t, m, "pr-title-20", true)
		require.Equal(t, before, after, "the list shouldn't scroll")
	}
}

func TestMouse_WheelOverPreviewScrollsIt(t *testing.T) {
	for _, position := range []string{"right", "bottom"} {
		t.Run(position, func(t *testing.T) {
			m := newMouseTestModel(t, position)
			m.sidebar.SetContent(strings.Repeat("preview line\n", 200))

			x, y := screenPos(t, m, "preview line", false)
			wheel(m, x, y, tea.MouseWheelDown)
			wheel(m, x, y, tea.MouseWheelDown)
			require.Equal(t, 6, m.sidebar.YOffset())
			require.Equal(t, 0, m.getCurrSection().CurrRow(),
				"scrolling the preview shouldn't move the selection")

			wheel(m, x, y, tea.MouseWheelUp)
			require.Equal(t, 3, m.sidebar.YOffset())
		})
	}
}

func TestMouse_ClickSectionTab(t *testing.T) {
	m := newMouseTestModel(t, "right")

	x, y := screenPos(t, m, "Second", false)
	require.Eventually(t, func() bool {
		return !zone.Get("section-tab-1").IsZero()
	}, time.Second, 10*time.Millisecond)

	click(m, x, y)
	require.Equal(t, 1, m.currSectionId)
	require.Equal(t, 1, m.tabs.CurrSectionId())
}

func TestMouse_ClickPreviewTab(t *testing.T) {
	m := newMouseTestModel(t, "right")

	x, y := screenPos(t, m, "Checks", false)
	require.Eventually(t, func() bool {
		return !zone.Get("pr-tab-3").IsZero()
	}, time.Second, 10*time.Millisecond)

	click(m, x, y)
	require.Equal(t, 3, m.prView.TabAt(tea.MouseClickMsg{X: x, Y: y}))
	require.Contains(t, m.prView.SelectedTab(), "Checks")
}

func TestMouse_IgnoredWhileSearching(t *testing.T) {
	m := newMouseTestModel(t, "right")
	m.getCurrSection().SetIsSearching(true)

	x, y := screenPos(t, m, "pr-title-3", true)
	click(m, x, y)
	wheel(m, x, y, tea.MouseWheelDown)
	require.Equal(t, 0, m.getCurrSection().CurrRow())
}
