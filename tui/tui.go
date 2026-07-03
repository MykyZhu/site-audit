package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"site-audit/audit"
	"site-audit/crawler"
	"site-audit/history"
	"site-audit/logger"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ── Colours ───────────────────────────────────────────────────────────────────

var (
	colAccent = lipgloss.Color("12")
	colGreen  = lipgloss.Color("10")
	colRed    = lipgloss.Color("9")
	colMuted  = lipgloss.Color("8")
	colText   = lipgloss.Color("7")
	colBorder = lipgloss.Color("8")
	colActive = lipgloss.Color("12")
)

// ── Styles ────────────────────────────────────────────────────────────────────

var (
	stylePanelActive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colActive)
	stylePanelInactive = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colBorder)

	styleTitle        = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleMuted        = lipgloss.NewStyle().Foreground(colMuted)
	styleGreen        = lipgloss.NewStyle().Foreground(colGreen)
	styleRed          = lipgloss.NewStyle().Foreground(colRed)
	styleText         = lipgloss.NewStyle().Foreground(colText)
	styleHistSelected = lipgloss.NewStyle().Bold(true).Foreground(colAccent)
	styleHistNormal   = lipgloss.NewStyle().Foreground(colText)
	stylePageSelected = lipgloss.NewStyle().Bold(true).Foreground(colGreen)
	styleHelp         = lipgloss.NewStyle().Foreground(colMuted).Padding(0, 1)
	styleSavedSel     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("11")) // yellow
)

// ── View states ───────────────────────────────────────────────────────────────

type viewState int

const (
	stateIdle      viewState = iota
	stateCrawling
	stateSelecting
	stateAuditing
	stateResults
)

// ── Focus ─────────────────────────────────────────────────────────────────────

type focusArea int

const (
	focusURL       focusArea = iota
	focusSaved                // pinned URLs
	focusHistory              // past scan logs
	focusFinderBtn
	focusRight
)

// ── Messages ──────────────────────────────────────────────────────────────────

type crawlDoneMsg struct {
	pages []crawler.Page
	err   error
}

type auditNextMsg struct {
	pages []crawler.Page
	index int
}

type auditPageDoneMsg struct {
	index  int
	result audit.PageResult
	pages  []crawler.Page
}

type auditAllDoneMsg struct{}

// ── Model ─────────────────────────────────────────────────────────────────────

type Model struct {
	width  int
	height int

	state viewState
	focus focusArea

	// Left: URL input
	urlInput textinput.Model

	// Left: saved URLs
	savedEntries []history.SavedEntry
	savedCursor  int

	// Left: history
	historyEntries []history.Entry
	historyCursor  int

	// Right: page selector
	pageItems  []pageItem
	pageCursor int

	// Right: audit progress
	auditLines   []string
	auditResults []audit.PageResult
	auditTotal   int
	auditCurrent int

	// Right: log viewer
	logViewport viewport.Model
	logContent  string

	// Spinner
	spinner spinner.Model

	// Status bar
	statusMsg string
	statusErr bool
}

type pageItem struct {
	page     crawler.Page
	selected bool
}

func New() Model {
	// Reconcile history with actual files on disk before loading
	history.Sync()

	ti := textinput.New()
	ti.Placeholder = "https://yoursite.com"
	ti.Focus()
	ti.CharLimit = 256

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(colAccent)

	return Model{
		urlInput:       ti,
		spinner:        sp,
		logViewport:    viewport.New(80, 20),
		savedEntries:   history.LoadSaved(),
		historyEntries: history.Load(),
		focus:          focusURL,
		state:          stateIdle,
	}
}

// NewWithFile opens the TUI directly to a highlighted log file.
func NewWithFile(path string) (Model, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Model{}, err
	}
	m := New() // Sync() is called inside New()
	m.logContent = logger.Highlight(string(data))
	m.state = stateResults
	m.focus = focusRight
	m.statusMsg = "Viewing: " + filepath.Base(path)
	// viewport will be sized on first WindowSizeMsg
	return m, nil
}

// ── Init ──────────────────────────────────────────────────────────────────────

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

// ── Update ────────────────────────────────────────────────────────────────────

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeViewport()
		// If we opened with a file, set content now that viewport is sized
		if m.state == stateResults && m.logContent != "" {
			m.logViewport.SetContent(m.logContent)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case crawlDoneMsg:
		if msg.err != nil {
			m.state = stateIdle
			m.setError("Crawl error: " + msg.err.Error())
		} else if len(msg.pages) == 0 {
			m.state = stateIdle
			m.setError("No internal pages found.")
		} else {
			m.state = stateSelecting
			m.focus = focusRight
			m.pageItems = make([]pageItem, len(msg.pages))
			for i, p := range msg.pages {
				m.pageItems[i] = pageItem{page: p}
			}
			m.pageCursor = 0
			m.setStatus(fmt.Sprintf("Found %d page(s) — space: toggle  a: all  enter: audit", len(msg.pages)))
		}

	case auditNextMsg:
		if msg.index >= len(msg.pages) {
			cmds = append(cmds, func() tea.Msg { return auditAllDoneMsg{} })
		} else {
			m.auditCurrent = msg.index
			cmds = append(cmds, doAuditOne(msg.pages, msg.index))
		}

	case auditPageDoneMsg:
		m.auditResults = append(m.auditResults, msg.result)
		r := msg.result
		var totalB int64
		for _, res := range r.Resources {
			totalB += res.TransferB
		}
		if r.Error != "" {
			m.auditLines[msg.index] = styleRed.Render(
				fmt.Sprintf("  [%d/%d] ✗  %s", msg.index+1, m.auditTotal, r.URL),
			) + "\n" + styleMuted.Render("         "+r.Error)
		} else {
			m.auditLines[msg.index] = styleGreen.Render(
				fmt.Sprintf("  [%d/%d] ✓  %s", msg.index+1, m.auditTotal, r.URL),
			) + "\n" + styleMuted.Render(fmt.Sprintf(
				"         %dms · %d resources · %s",
				r.TotalTimeMs, len(r.Resources), formatBytes(totalB),
			))
		}
		cmds = append(cmds, func() tea.Msg {
			return auditNextMsg{pages: msg.pages, index: msg.index + 1}
		})

	case auditAllDoneMsg:
		m.state = stateResults
		m.focus = focusRight

		home, _ := os.UserHomeDir()
		outDir := filepath.Join(home, "Desktop", "_Logs")
		baseName := siteBaseName(m.urlInput.Value()) + "_" + time.Now().Format("2006-01-02_15-04-05")
		logPath := filepath.Join(outDir, baseName+".log")

		if err := logger.Write(m.auditResults, outDir, baseName); err != nil {
			m.setError("Error writing log: " + err.Error())
		} else {
			history.Add(siteBaseName(m.urlInput.Value()), logPath)
			m.historyEntries = history.Load()
			m.setStatus("Saved → " + logPath)
		}

		m.logContent = logger.Highlight(logger.BuildContent(m.auditResults))
		m.resizeViewport()
		m.logViewport.SetContent(m.logContent)
		m.logViewport.GotoTop()
		// Reset URL input so it's ready for the next scan
		m.urlInput.SetValue("")
		m.urlInput.Focus()

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "tab":
			m = m.rotateFocus(1)
			return m, nil
		case "shift+tab":
			m = m.rotateFocus(-1)
			return m, nil
		default:
			var cmd tea.Cmd
			m, cmd = m.handleKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case tea.MouseMsg:
		if m.state == stateResults {
			var cmd tea.Cmd
			m.logViewport, cmd = m.logViewport.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	if m.state == stateCrawling || m.state == stateAuditing {
		cmds = append(cmds, m.spinner.Tick)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) handleKey(msg tea.KeyMsg) (Model, tea.Cmd) {
	// ── Panels accessible from any state ─────────────────────────────────────

	switch m.focus {

	case focusSaved:
		switch msg.String() {
		case "up", "k":
			if m.savedCursor > 0 {
				m.savedCursor--
			} else {
				m.focus = focusURL
				m.urlInput.Focus()
			}
		case "down", "j":
			if m.savedCursor < len(m.savedEntries)-1 {
				m.savedCursor++
			}
		case "enter", " ":
			if len(m.savedEntries) > 0 {
				e := m.savedEntries[m.savedCursor]
				return m, m.startScan(e.URL)
			}
		case "d":
			if len(m.savedEntries) > 0 {
				history.RemoveSaved(m.savedCursor)
				m.savedEntries = history.LoadSaved()
				if m.savedCursor >= len(m.savedEntries) && m.savedCursor > 0 {
					m.savedCursor--
				}
				m.setStatus("Removed from saved.")
			}
		}
		return m, nil

	case focusHistory:
		switch msg.String() {
		case "up", "k":
			if m.historyCursor > 0 {
				m.historyCursor--
			} else {
				m.focus = focusSaved
			}
		case "down", "j":
			if m.historyCursor < len(m.historyEntries)-1 {
				m.historyCursor++
			}
		case "enter", " ":
			// Open the log file for this history entry
			if len(m.historyEntries) > 0 {
				m.openLogFile(m.historyEntries[m.historyCursor].LogPath)
			}
		case "s":
			// Save this URL to the Saved pane
			if len(m.historyEntries) > 0 {
				e := m.historyEntries[m.historyCursor]
				url := "https://" + e.SiteName
				history.AddSaved(e.SiteName, url)
				m.savedEntries = history.LoadSaved()
				m.setStatus("Saved " + e.SiteName + " to Saved.")
			}
		case "d":
			// Delete this history entry
			if len(m.historyEntries) > 0 {
				history.Remove(m.historyCursor)
				m.historyEntries = history.Load()
				if m.historyCursor >= len(m.historyEntries) && m.historyCursor > 0 {
					m.historyCursor--
				}
				m.setStatus("Removed from history.")
			}
		}
		return m, nil

	case focusFinderBtn:
		switch msg.String() {
		case "enter", " ":
			home, _ := os.UserHomeDir()
			openInFinder(filepath.Join(home, "Desktop", "_Logs"))
		}
		return m, nil
	}

	// ── URL input works from any state ──────────────────────────────────────

	if m.focus == focusURL {
		switch msg.String() {
		case "enter":
			url := strings.TrimSpace(m.urlInput.Value())
			if url == "" {
				m.setError("Please enter a URL")
				return m, nil
			}
			if !strings.HasPrefix(url, "http") {
				url = "https://" + url
				m.urlInput.SetValue(url)
			}
			return m, m.startScan(url)
		case "down":
			if len(m.savedEntries) > 0 {
				m.focus = focusSaved
				m.urlInput.Blur()
			} else if len(m.historyEntries) > 0 {
				m.focus = focusHistory
				m.urlInput.Blur()
			}
		default:
			var cmd tea.Cmd
			m.urlInput, cmd = m.urlInput.Update(msg)
			return m, cmd
		}
	}

	// ── State-specific ────────────────────────────────────────────────────────

	switch m.state {
	case stateSelecting:
		switch msg.String() {
		case " ":
			m.pageItems[m.pageCursor].selected = !m.pageItems[m.pageCursor].selected
		case "up", "k":
			if m.pageCursor > 0 {
				m.pageCursor--
			}
		case "down", "j":
			if m.pageCursor < len(m.pageItems)-1 {
				m.pageCursor++
			}
		case "a":
			for i := range m.pageItems {
				m.pageItems[i].selected = true
			}
		case "n":
			for i := range m.pageItems {
				m.pageItems[i].selected = false
			}
		case "enter":
			var chosen []crawler.Page
			for _, it := range m.pageItems {
				if it.selected {
					chosen = append(chosen, it.page)
				}
			}
			if len(chosen) == 0 {
				m.setError("Select at least one page (space to toggle, a for all)")
				return m, nil
			}
			m.state = stateAuditing
			m.auditTotal = len(chosen)
			m.auditCurrent = 0
			m.auditLines = make([]string, len(chosen))
			m.auditResults = nil
			for i := range m.auditLines {
				m.auditLines[i] = styleMuted.Render(fmt.Sprintf("  [%d/%d] pending…", i+1, len(chosen)))
			}
			m.setStatus(fmt.Sprintf("Auditing %d page(s)…", len(chosen)))
			return m, func() tea.Msg { return auditNextMsg{pages: chosen, index: 0} }
		case "esc":
			m.state = stateIdle
			m.focus = focusURL
			m.urlInput.Focus()
		}

	case stateResults:
		switch msg.String() {
		case "q", "esc":
			m.state = stateIdle
			m.focus = focusURL
			m.urlInput.Focus()
		default:
			if m.focus == focusRight {
				var cmd tea.Cmd
				m.logViewport, cmd = m.logViewport.Update(msg)
				return m, cmd
			}
		}
	}

	return m, nil
}

// startScan kicks off a crawl for the given URL
func (m *Model) startScan(url string) tea.Cmd {
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}
	m.state = stateCrawling
	m.focus = focusRight
	m.urlInput.Blur()
	m.setStatus("Scanning " + url + "…")
	return doCrawl(url)
}

// openLogFile loads and displays a log file in the right panel
func (m *Model) openLogFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		m.setError("Could not open log: " + err.Error())
		return
	}
	m.logContent = logger.Highlight(string(data))
	m.resizeViewport()
	m.logViewport.SetContent(m.logContent)
	m.logViewport.GotoTop()
	m.state = stateResults
	m.focus = focusRight
	m.setStatus("Viewing: " + filepath.Base(path))
}

func (m Model) rotateFocus(dir int) Model {
	areas := []focusArea{focusURL, focusSaved, focusHistory, focusFinderBtn, focusRight}
	cur := 0
	for i, a := range areas {
		if a == m.focus {
			cur = i
		}
	}
	next := (cur + dir + len(areas)) % len(areas)
	m.focus = areas[next]
	if m.focus == focusURL {
		m.urlInput.Focus()
	} else {
		m.urlInput.Blur()
	}
	return m
}

func (m *Model) resizeViewport() {
	w := m.rightW() - 4
	h := m.panelH() - 2
	if w < 10 {
		w = 10
	}
	if h < 5 {
		h = 5
	}
	m.logViewport.Width = w
	m.logViewport.Height = h
}

func (m *Model) setStatus(s string) { m.statusMsg = s; m.statusErr = false }
func (m *Model) setError(s string)  { m.statusMsg = s; m.statusErr = true }

// ── View ──────────────────────────────────────────────────────────────────────

func (m Model) View() string {
	if m.width == 0 {
		return "Loading…"
	}
	left := m.viewLeft()
	right := m.viewRight()
	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	return lipgloss.JoinVertical(lipgloss.Left, panels, m.viewHelp())
}

func (m Model) leftW() int  { return 38 }
func (m Model) rightW() int { return m.width - m.leftW() }
func (m Model) panelH() int {
	h := m.height - 1
	if h < 10 {
		return 10
	}
	return h
}

func (m Model) viewLeft() string {
	// 5 stacked panels in left column, each with a border (2 rows each = 10 total overhead)
	// Fixed heights: URL=5, Saved=dynamic, History=dynamic, Finder=3
	// Give Saved and History equal share of what's left
	const urlOuterH    = 5
	const finderOuterH = 3
	remaining := m.panelH() - urlOuterH - finderOuterH
	// Split remaining between Saved and History
	savedOuterH := remaining / 2
	histOuterH := remaining - savedOuterH

	// ── URL
	urlBorder := stylePanelInactive
	if m.focus == focusURL {
		urlBorder = stylePanelActive
	}
	urlBox := urlBorder.
		Width(m.leftW() - 2).
		Height(urlOuterH - 2).
		Render(styleTitle.Render("URL") + "\n" + m.urlInput.View())

	// ── Saved
	savedBorder := stylePanelInactive
	if m.focus == focusSaved {
		savedBorder = stylePanelActive
	}
	savedInnerH := savedOuterH - 4
	if savedInnerH < 1 {
		savedInnerH = 1
	}
	savedBox := savedBorder.
		Width(m.leftW() - 2).
		Height(savedOuterH - 2).
		Render(m.renderSavedContent(savedInnerH))

	// ── History
	histBorder := stylePanelInactive
	if m.focus == focusHistory {
		histBorder = stylePanelActive
	}
	histInnerH := histOuterH - 4
	if histInnerH < 1 {
		histInnerH = 1
	}
	histBox := histBorder.
		Width(m.leftW() - 2).
		Height(histOuterH - 2).
		Render(m.renderHistoryContent(histInnerH))

	// ── Finder button
	finderBorder := stylePanelInactive
	if m.focus == focusFinderBtn {
		finderBorder = stylePanelActive
	}
	finderLabel := "  ↗  Open logs folder in Finder"
	if m.focus == focusFinderBtn {
		finderLabel = styleGreen.Render("  ↗  Open logs folder in Finder")
	}
	finderBox := finderBorder.
		Width(m.leftW() - 2).
		Height(finderOuterH - 2).
		Render(finderLabel)

	return lipgloss.JoinVertical(lipgloss.Left, urlBox, savedBox, histBox, finderBox)
}

func (m Model) renderSavedContent(innerH int) string {
	lines := []string{styleTitle.Render("SAVED")}
	if len(m.savedEntries) == 0 {
		lines = append(lines, styleMuted.Render("  Press s on a history item"))
	} else {
		start := 0
		if m.savedCursor >= innerH {
			start = m.savedCursor - innerH + 1
		}
		end := start + innerH
		if end > len(m.savedEntries) {
			end = len(m.savedEntries)
		}
		for i := start; i < end; i++ {
			e := m.savedEntries[i]
			name := truncate(e.SiteName, 22)
			if i == m.savedCursor && m.focus == focusSaved {
				lines = append(lines, styleSavedSel.Render(fmt.Sprintf("▶ %-22s", name)))
			} else {
				lines = append(lines, styleHistNormal.Render(fmt.Sprintf("  %-22s", name)))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderHistoryContent(innerH int) string {
	lines := []string{styleTitle.Render("HISTORY")}
	if len(m.historyEntries) == 0 {
		lines = append(lines, styleMuted.Render("  No history yet"))
	} else {
		start := 0
		if m.historyCursor >= innerH {
			start = m.historyCursor - innerH + 1
		}
		end := start + innerH
		if end > len(m.historyEntries) {
			end = len(m.historyEntries)
		}
		for i := start; i < end; i++ {
			e := m.historyEntries[i]
			name := truncate(e.SiteName, 16)
			date := styleMuted.Render(e.Date.Format("01-02 15:04"))
			if i == m.historyCursor && m.focus == focusHistory {
				lines = append(lines,
					styleHistSelected.Render(fmt.Sprintf("▶ %-16s", name))+" "+date)
			} else {
				lines = append(lines,
					styleHistNormal.Render(fmt.Sprintf("  %-16s", name))+" "+date)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) viewRight() string {
	border := stylePanelInactive
	if m.focus == focusRight {
		border = stylePanelActive
	}
	return border.
		Width(m.rightW() - 2).
		Height(m.panelH() - 2).
		Render(m.viewRightInner())
}

func (m Model) viewRightInner() string {
	switch m.state {
	case stateIdle:
		return strings.Join([]string{
			styleTitle.Render("SITE AUDIT"),
			"",
			styleMuted.Render("  Enter a URL on the left and press enter to scan."),
			styleMuted.Render("  Pick a saved URL to re-run a scan."),
			styleMuted.Render("  Select a history entry to view its log."),
			"",
			styleText.Render("  tab / shift+tab to move between panels."),
			styleText.Render("  s: save to Saved  ·  d: delete  ·  enter: open/run"),
		}, "\n")

	case stateCrawling:
		return fmt.Sprintf("  %s  %s", m.spinner.View(),
			styleMuted.Render("Scanning for internal pages…"))

	case stateSelecting:
		lines := []string{styleTitle.Render("SELECT PAGES"), ""}
		for i, it := range m.pageItems {
			check := styleMuted.Render("○")
			if it.selected {
				check = styleGreen.Render("◉")
			}
			label := it.page.URL
			if it.page.Title != "" && it.page.Title != it.page.URL {
				label += styleMuted.Render("  (" + truncate(it.page.Title, 40) + ")")
			}
			prefix := "  "
			lineStyle := styleText
			if i == m.pageCursor {
				prefix = stylePageSelected.Render("▶ ")
				lineStyle = stylePageSelected
			}
			lines = append(lines, prefix+check+"  "+lineStyle.Render(label))
		}
		return strings.Join(lines, "\n")

	case stateAuditing:
		lines := []string{styleTitle.Render("AUDITING"), ""}
		for i, line := range m.auditLines {
			if i == m.auditCurrent && i >= len(m.auditResults) {
				lines = append(lines,
					fmt.Sprintf("  %s  %s", m.spinner.View(),
						styleMuted.Render(fmt.Sprintf("[%d/%d] auditing…", i+1, m.auditTotal))))
			} else {
				lines = append(lines, line)
			}
		}
		return strings.Join(lines, "\n")

	case stateResults:
		return m.logViewport.View()
	}
	return ""
}

func (m Model) viewHelp() string {
	var hints []string
	switch m.state {
	case stateIdle:
		switch m.focus {
		case focusHistory:
			hints = []string{"↑↓: navigate", "enter: view log", "s: save URL", "d: delete", "tab: switch"}
		case focusSaved:
			hints = []string{"↑↓: navigate", "enter: run scan", "d: delete", "tab: switch"}
		default:
			hints = []string{"tab: switch panel", "↑↓: navigate", "enter: run/open", "ctrl+c: quit"}
		}
	case stateCrawling:
		hints = []string{"scanning…"}
	case stateSelecting:
		hints = []string{"↑↓/jk: navigate", "space: toggle", "a: all", "n: none", "enter: audit", "esc: back"}
	case stateAuditing:
		hints = []string{"auditing — please wait"}
	case stateResults:
		hints = []string{"↑↓ pgup/pgdn: scroll", "tab: switch panel", "q/esc: back"}
	}

	statusStyle := styleMuted
	if m.statusErr {
		statusStyle = styleRed
	}
	status := statusStyle.Render(m.statusMsg)
	help := styleHelp.Render(strings.Join(hints, "  ·  "))
	gap := m.width - lipgloss.Width(help) - lipgloss.Width(status) - 2
	if gap < 1 {
		gap = 1
	}
	return help + strings.Repeat(" ", gap) + status
}

// ── Commands ──────────────────────────────────────────────────────────────────

func doCrawl(url string) tea.Cmd {
	return func() tea.Msg {
		pages, err := crawler.Crawl(url)
		return crawlDoneMsg{pages: pages, err: err}
	}
}

func doAuditOne(pages []crawler.Page, index int) tea.Cmd {
	return func() tea.Msg {
		result := audit.Run(pages[index].URL)
		return auditPageDoneMsg{index: index, result: result, pages: pages}
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func openInFinder(path string) {
	exec.Command("open", path).Start()
}

func siteBaseName(rawURL string) string {
	s := strings.TrimPrefix(rawURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	if i := strings.Index(s, "/"); i != -1 {
		s = s[:i]
	}
	s = strings.TrimPrefix(s, "www.")
	re := regexp.MustCompile(`[^a-zA-Z0-9.\-]`)
	return re.ReplaceAllString(s, "_")
}

func formatBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
