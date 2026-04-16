package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhrsntrk/portflix/internal/scanner"
)

type portsScreen int

const (
	screenList portsScreen = iota
	screenDetail
	screenConfirm
)

type PortsModel struct {
	ports        []scanner.Port
	cursor       int
	showAll      bool
	loading      bool
	refreshing   bool
	refreshFrame int
	err          error
	width        int
	height       int
	screen       portsScreen
	sp           spinner.Model
	msg          string
	msgOK        bool
	// set when user presses 'l' — caller should launch logs for this port
	LogsPort int
}

type portsLoadedMsg struct {
	ports []scanner.Port
	err   error
}
type killDoneMsg struct{ pid int; ok bool }
type autoRefreshMsg struct{}
type pulseTickMsg struct{}

func NewPortsModel(showAll bool) PortsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cAccent)
	return PortsModel{showAll: showAll, loading: true, sp: sp}
}

func loadPorts(showAll bool) tea.Cmd {
	return func() tea.Msg {
		ports, err := scanner.GetListeningPorts(false)
		if err != nil {
			return portsLoadedMsg{err: err}
		}
		if !showAll {
			var filtered []scanner.Port
			for _, p := range ports {
				if scanner.IsDevProcess(p.ProcessName, p.Command) {
					filtered = append(filtered, p)
				}
			}
			ports = filtered
		}
		return portsLoadedMsg{ports: ports}
	}
}

func killCmd(pid int, force bool) tea.Cmd {
	return func() tea.Msg {
		return killDoneMsg{pid, scanner.KillProcess(pid, force)}
	}
}

func autoRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return autoRefreshMsg{} })
}

func pulseTick() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(time.Time) tea.Msg { return pulseTickMsg{} })
}

func (m PortsModel) Init() tea.Cmd {
	return tea.Batch(loadPorts(m.showAll), m.sp.Tick)
}

func (m PortsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}

	case pulseTickMsg:
		if m.refreshing {
			m.refreshFrame++
			return m, pulseTick()
		}

	case portsLoadedMsg:
		m.loading = false
		m.refreshing = false
		m.refreshFrame = 0
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.ports = msg.ports
			if m.cursor >= len(m.ports) && len(m.ports) > 0 {
				m.cursor = len(m.ports) - 1
			}
		}
		return m, autoRefresh()

	case killDoneMsg:
		if msg.ok {
			m.msg = fmt.Sprintf("✓ Killed PID %d", msg.pid)
			m.msgOK = true
		} else {
			m.msg = fmt.Sprintf("✕ Failed — try: sudo kill -9 %d", msg.pid)
			m.msgOK = false
		}
		m.screen = screenList
		m.loading = true
		return m, tea.Batch(loadPorts(m.showAll), m.sp.Tick)

	case autoRefreshMsg:
		if !m.loading && !m.refreshing && m.screen == screenList {
			m.refreshing = true
			m.refreshFrame = 0
			return m, tea.Batch(loadPorts(m.showAll), pulseTick())
		}
		return m, autoRefresh()

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "ctrl+c" || msg.String() == "q" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch m.screen {
		case screenList:
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "up", "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "down", "j":
				if m.cursor < len(m.ports)-1 {
					m.cursor++
				}
			case "r":
				m.loading = true
				return m, tea.Batch(loadPorts(m.showAll), m.sp.Tick)
			case "a":
				m.showAll = !m.showAll
				m.loading = true
				return m, tea.Batch(loadPorts(m.showAll), m.sp.Tick)
			case "enter", "d":
				if len(m.ports) > 0 {
					m.screen = screenDetail
				}
			case "K":
				if len(m.ports) > 0 {
					m.screen = screenConfirm
					m.msg = ""
				}
			case "l":
				if len(m.ports) > 0 {
					m.LogsPort = m.ports[m.cursor].Port
					return m, tea.Quit
				}
			}

		case screenDetail:
			switch msg.String() {
			case "q", "esc", "backspace":
				m.screen = screenList
			case "K":
				m.screen = screenConfirm
				m.msg = ""
			}

		case screenConfirm:
			switch msg.String() {
			case "y", "Y":
				if len(m.ports) > 0 {
					return m, killCmd(m.ports[m.cursor].PID, false)
				}
			case "f", "F":
				if len(m.ports) > 0 {
					return m, killCmd(m.ports[m.cursor].PID, true)
				}
			case "n", "N", "esc", "q":
				m.screen = screenList
			}
		}
	}
	return m, nil
}

func (m PortsModel) View() string {
	if m.loading && len(m.ports) == 0 {
		return "\n  " + m.sp.View() + " scanning ports...\n"
	}
	switch m.screen {
	case screenDetail:
		return m.viewDetail()
	case screenConfirm:
		return m.viewList() + "\n" + m.viewConfirm()
	}
	return m.viewList()
}

var sep = sMuted.Render("  │  ")

const sepW = 5 // display width of sep

// cols holds the active column widths; zero means the column is hidden.
type cols struct{ port, proc, pid, proj, fw, up, st int }

func computeCols(width int) cols {
	if width == 0 {
		width = 120
	}
	avail := width - 2 // subtract cursor prefix "▶ "

	// All 7 columns
	if avail >= 7+12+7+22+12+9+12+6*sepW {
		return cols{7, 12, 7, 22, 12, 9, 12}
	}
	// Drop PID
	if avail >= 7+12+22+12+9+12+5*sepW {
		return cols{7, 12, 0, 22, 12, 9, 12}
	}
	// Drop PID, shrink PROJECT to fit (6 cols, 5 seps)
	projW := avail - (7 + 12 + 12 + 9 + 12) - 5*sepW
	if projW >= 10 {
		if projW > 22 {
			projW = 22
		}
		return cols{7, 12, 0, projW, 12, 9, 12}
	}
	// Drop PID + PROJECT
	if avail >= 7+12+12+9+12+4*sepW {
		return cols{7, 12, 0, 0, 12, 9, 12}
	}
	// Drop PID + PROJECT + UPTIME
	if avail >= 7+12+12+12+3*sepW {
		return cols{7, 12, 0, 0, 12, 0, 12}
	}
	// Minimum: PORT, PROCESS, STATUS
	return cols{7, 12, 0, 0, 0, 0, 12}
}

func (c cols) dividerWidth() int {
	w := c.port + c.proc + c.st
	nseps := 2
	for _, v := range []int{c.pid, c.proj, c.fw, c.up} {
		if v > 0 {
			w += v
			nseps++
		}
	}
	return w + nseps*sepW
}

func (m PortsModel) viewList() string {
	var b strings.Builder

	b.WriteString(renderBanner() + "\n\n")

	// Status message
	if m.msg != "" {
		style := sRed
		if m.msgOK {
			style = sGreen
		}
		b.WriteString("  " + style.Render(m.msg) + "\n\n")
	}

	c := computeCols(m.width)

	if len(m.ports) == 0 {
		b.WriteString("  " + sMuted.Render("No active ports.") + "\n")
		b.WriteString("  " + sMuted.Render("Press r to refresh, a to show all.") + "\n")
	} else {
		hdr := "  " + renderHeader(c)
		divLine := "  " + sDivider.Render(strings.Repeat("─", c.dividerWidth()))
		b.WriteString(hdr + "\n" + divLine + "\n")

		for i, p := range m.ports {
			b.WriteString(m.renderRow(i, p, c) + "\n")
		}
	}

	// Footer
	b.WriteString("\n")
	mode := "[dev mode]"
	if m.showAll {
		mode = "[all mode]"
	}
	indicator := pulseCharIdle()
	if m.refreshing {
		indicator = pulseChar(m.refreshFrame)
	}
	b.WriteString("  " + sMuted.Render(fmt.Sprintf("%d port%s  %s", len(m.ports), plural(len(m.ports)), mode)) + "  " + indicator + "\n")
	b.WriteString("  " + renderHints(m.width, [][2]string{
		{"↑↓/jk", "nav"},
		{"enter", "detail"},
		{"K", "kill"},
		{"l", "logs"},
		{"r", "refresh"},
		{"a", "all"},
		{"q", "quit"},
	}) + "\n")
	return b.String()
}

func renderHeader(c cols) string {
	h := sColHeader
	parts := []string{cell(h.Render("PORT"), c.port), cell(h.Render("PROCESS"), c.proc)}
	if c.pid > 0 {
		parts = append(parts, cell(h.Render("PID"), c.pid))
	}
	if c.proj > 0 {
		parts = append(parts, cell(h.Render("PROJECT"), c.proj))
	}
	if c.fw > 0 {
		parts = append(parts, cell(h.Render("FRAMEWORK"), c.fw))
	}
	if c.up > 0 {
		parts = append(parts, cell(h.Render("UPTIME"), c.up))
	}
	parts = append(parts, cell(h.Render("STATUS"), c.st))
	return strings.Join(parts, sep)
}

func (m PortsModel) renderRow(i int, p scanner.Port, c cols) string {
	sel := i == m.cursor
	portStr := fmt.Sprintf(":%d", p.Port)
	pidStr := fmt.Sprintf("%d", p.PID)
	bg := sSelected

	colVal := func(plain, styled string, w int) string {
		if sel {
			return cell(bg.Render(plain), w)
		}
		return cell(styled, w)
	}

	parts := []string{
		func() string {
			if sel {
				return cell(bg.Bold(true).Render(portStr), c.port)
			}
			return cell(sBold.Render(portStr), c.port)
		}(),
		colVal(p.ProcessName, sWhite.Render(p.ProcessName), c.proc),
	}
	if c.pid > 0 {
		parts = append(parts, colVal(pidStr, sMuted.Render(pidStr), c.pid))
	}
	if c.proj > 0 {
		proj := coalesce(p.ProjectName, "—")
		parts = append(parts, colVal(proj, sBlue.Render(proj), c.proj))
	}
	if c.fw > 0 {
		fw := fwColored(p.Framework)
		if sel {
			parts = append(parts, cell(bg.Render(p.Framework), c.fw))
		} else {
			parts = append(parts, cell(fw, c.fw))
		}
	}
	if c.up > 0 {
		up := coalesce(p.Uptime, "—")
		parts = append(parts, colVal(up, sYellow.Render(up), c.up))
	}
	parts = append(parts, func() string {
		if sel {
			return cell(bg.Render(statusCell(string(p.Status))), c.st)
		}
		return cell(statusCell(string(p.Status)), c.st)
	}())

	cursor := "  "
	if sel {
		cursor = sAccent.Render("▶ ")
	}
	return cursor + strings.Join(parts, sep)
}

func (m PortsModel) viewDetail() string {
	if len(m.ports) == 0 {
		return ""
	}
	p := m.ports[m.cursor]
	var b strings.Builder

	b.WriteString(renderBanner() + "\n")
	b.WriteString("  " + sMuted.Render("port detail") + "\n\n")

	row := func(label, value string) {
		b.WriteString("  " + sMuted.Render(pad(label, 16)) + value + "\n")
	}

	b.WriteString("  " + sBold.Render(fmt.Sprintf(":%d", p.Port)) + "\n")
	b.WriteString("  " + sDivider.Render(strings.Repeat("─", 40)) + "\n\n")

	row("Process", sBold.Render(p.ProcessName))
	row("PID", sMuted.Render(fmt.Sprintf("%d", p.PID)))
	row("Status", statusCell(string(p.Status)))
	row("Framework", fwColored(coalesce(p.Framework, "—")))
	row("Memory", sGreen.Render(coalesce(p.Memory, "—")))
	row("Uptime", sYellow.Render(coalesce(p.Uptime, "—")))
	if !p.StartTime.IsZero() {
		row("Started", sMuted.Render(p.StartTime.Format("Jan 2 15:04:05")))
	}

	b.WriteString("\n  " + sAccent.Render("Location") + "\n")
	b.WriteString("  " + sDivider.Render(strings.Repeat("─", 40)) + "\n\n")
	row("Directory", sBlue.Render(coalesce(p.CWD, "—")))
	row("Project", sWhite.Render(coalesce(p.ProjectName, "—")))
	if p.GitBranch != "" {
		row("Branch", lipgloss.NewStyle().Foreground(cMagenta).Render(p.GitBranch))
	}

	b.WriteString("\n  " + sMuted.Render("K") + " kill  " +
		sMuted.Render("esc") + " back\n")
	return b.String()
}

func (m PortsModel) viewConfirm() string {
	if len(m.ports) == 0 {
		return ""
	}
	p := m.ports[m.cursor]
	prompt := fmt.Sprintf("Kill :%d — %s (PID %d)?  ", p.Port, p.ProcessName, p.PID)
	return "  " + sYellow.Render(prompt) +
		sGreen.Render("y") + sMuted.Render(" yes  ") +
		sRed.Render("f") + sMuted.Render(" force  ") +
		sMuted.Render("n") + sMuted.Render(" cancel") + "\n"
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// RunPorts runs the interactive ports TUI and returns a logs port if the user
// pressed 'l' (0 means no logs requested).
func RunPorts(showAll bool) int {
	m := NewPortsModel(showAll)
	p := tea.NewProgram(m, tea.WithAltScreen())
	result, err := p.Run()
	if err != nil {
		return 0
	}
	if pm, ok := result.(PortsModel); ok {
		return pm.LogsPort
	}
	return 0
}
