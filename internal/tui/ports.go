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
	ports      []scanner.Port
	cursor     int
	showAll    bool
	loading    bool
	refreshing bool
	err        error
	width      int
	height     int
	screen     portsScreen
	sp         spinner.Model
	msg        string
	msgOK      bool
	// set when user presses 'l' — caller should launch logs for this port
	LogsPort int
}

type portsLoadedMsg struct {
	ports []scanner.Port
	err   error
}
type killDoneMsg struct{ pid int; ok bool }
type autoRefreshMsg struct{}

func NewPortsModel(showAll bool) PortsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cCyan)
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

	case portsLoadedMsg:
		m.loading = false
		m.refreshing = false
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
			return m, loadPorts(m.showAll)
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

const (
	wPort  = 7
	wProc  = 12
	wPID   = 7
	wProj  = 22
	wFW    = 12
	wUp    = 9
	wSt    = 12
)

var sep = sMuted.Render("  │  ")

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

	if len(m.ports) == 0 {
		b.WriteString("  " + sMuted.Render("No active ports.") + "\n")
		b.WriteString("  " + sMuted.Render("Press r to refresh, a to show all.") + "\n")
	} else {
		// Header
		hdr := "  " + renderHeader()
		divLine := "  " + sDivider.Render(strings.Repeat("─",
			wPort+wProc+wPID+wProj+wFW+wUp+wSt+lipgloss.Width(sep)*6))
		b.WriteString(hdr + "\n" + divLine + "\n")

		for i, p := range m.ports {
			b.WriteString(m.renderRow(i, p) + "\n")
		}
	}

	// Footer
	b.WriteString("\n")
	summary := fmt.Sprintf("%d port%s", len(m.ports), plural(len(m.ports)))
	if !m.showAll {
		summary += "  dev only"
	}
	if m.refreshing {
		summary += "  · refreshing"
	}
	b.WriteString("  " + sMuted.Render(summary) + "\n")
	hints := sMuted.Render("↑↓/jk") + " nav  " +
		sMuted.Render("enter") + " detail  " +
		sMuted.Render("K") + " kill  " +
		sMuted.Render("l") + " logs  " +
		sMuted.Render("r") + " refresh  " +
		sMuted.Render("a") + " all  " +
		sMuted.Render("q") + " quit"
	b.WriteString("  " + hints + "\n")
	return b.String()
}

func renderHeader() string {
	h := sColHeader
	return cell(h.Render("PORT"), wPort) + sep +
		cell(h.Render("PROCESS"), wProc) + sep +
		cell(h.Render("PID"), wPID) + sep +
		cell(h.Render("PROJECT"), wProj) + sep +
		cell(h.Render("FRAMEWORK"), wFW) + sep +
		cell(h.Render("UPTIME"), wUp) + sep +
		cell(h.Render("STATUS"), wSt)
}

func (m PortsModel) renderRow(i int, p scanner.Port) string {
	sel := i == m.cursor
	portStr := fmt.Sprintf(":%d", p.Port)
	pidStr := fmt.Sprintf("%d", p.PID)

	var portCell, procCell, pidCell, projCell, fwCell, upCell, stCell string

	if sel {
		bg := sSelected
		portCell = cell(bg.Bold(true).Render(portStr), wPort)
		procCell = cell(bg.Render(p.ProcessName), wProc)
		pidCell = cell(bg.Foreground(cMuted).Render(pidStr), wPID)
		projCell = cell(bg.Foreground(cBlue).Render(coalesce(p.ProjectName, "—")), wProj)
		fwCell = cell(bg.Render(fwColored(p.Framework)), wFW)
		upCell = cell(bg.Foreground(cYellow).Render(coalesce(p.Uptime, "—")), wUp)
		stCell = cell(bg.Render(statusCell(string(p.Status))), wSt)
	} else {
		portCell = cell(sBold.Render(portStr), wPort)
		procCell = cell(sWhite.Render(p.ProcessName), wProc)
		pidCell = cell(sMuted.Render(pidStr), wPID)
		projCell = cell(sBlue.Render(coalesce(p.ProjectName, "—")), wProj)
		fwCell = cell(fwColored(p.Framework), wFW)
		upCell = cell(sYellow.Render(coalesce(p.Uptime, "—")), wUp)
		stCell = cell(statusCell(string(p.Status)), wSt)
	}

	cursor := "  "
	if sel {
		cursor = sCyan.Render("▶ ")
	}
	return cursor + portCell + sep + procCell + sep + pidCell + sep +
		projCell + sep + fwCell + sep + upCell + sep + stCell
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

	b.WriteString("\n  " + sCyan.Render("Location") + "\n")
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
