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

type psModel struct {
	procs   []scanner.Process
	cursor  int
	showAll bool
	loading bool
	err     error
	sp      spinner.Model
}

type procsLoadedMsg struct {
	procs []scanner.Process
	err   error
}
type psRefreshMsg struct{}

func newPSModel(showAll bool) psModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cCyan)
	return psModel{showAll: showAll, loading: true, sp: sp}
}

func loadProcs(showAll bool) tea.Cmd {
	return func() tea.Msg {
		procs, err := scanner.GetAllProcesses()
		if err != nil {
			return procsLoadedMsg{err: err}
		}
		if !showAll {
			// Filter to dev processes only, collapse Docker
			var filtered []scanner.Process
			var dockerProcs []scanner.Process
			for _, p := range procs {
				if !scanner.IsDevProcess(p.ProcessName, p.Command) {
					continue
				}
				if strings.HasPrefix(p.ProcessName, "com.docke") ||
					strings.HasPrefix(p.ProcessName, "Docker") ||
					p.ProcessName == "docker" || p.ProcessName == "docker-sandbox" {
					dockerProcs = append(dockerProcs, p)
				} else {
					filtered = append(filtered, p)
				}
			}
			if len(dockerProcs) > 0 {
				var totalCPU float64
				var totalRSSKB int
				for _, d := range dockerProcs {
					totalCPU += d.CPU
					// approximate: parse memory back to KB
					totalRSSKB += approxKB(d.Memory)
				}
				filtered = append(filtered, scanner.Process{
					PID:         dockerProcs[0].PID,
					ProcessName: "Docker",
					Description: fmt.Sprintf("%d processes", len(dockerProcs)),
					CPU:         totalCPU,
					Memory:      scanner.FormatMemoryKB(totalRSSKB),
					Framework:   "Docker",
					Uptime:      dockerProcs[0].Uptime,
				})
			}
			procs = filtered
		}
		// Sort by CPU descending
		for i := 0; i < len(procs); i++ {
			for j := i + 1; j < len(procs); j++ {
				if procs[j].CPU > procs[i].CPU {
					procs[i], procs[j] = procs[j], procs[i]
				}
			}
		}
		return procsLoadedMsg{procs: procs}
	}
}

func psAutoRefresh() tea.Cmd {
	return tea.Tick(5*time.Second, func(time.Time) tea.Msg { return psRefreshMsg{} })
}

func (m psModel) Init() tea.Cmd {
	return tea.Batch(loadProcs(m.showAll), m.sp.Tick)
}

func (m psModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// future: use for responsive layout

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}

	case procsLoadedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.procs = msg.procs
			if m.cursor >= len(m.procs) && len(m.procs) > 0 {
				m.cursor = len(m.procs) - 1
			}
		}
		return m, psAutoRefresh()

	case psRefreshMsg:
		if !m.loading {
			m.loading = true
			return m, tea.Batch(loadProcs(m.showAll), m.sp.Tick)
		}
		return m, psAutoRefresh()

	case tea.KeyMsg:
		if m.loading {
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.procs)-1 {
				m.cursor++
			}
		case "r":
			m.loading = true
			return m, tea.Batch(loadProcs(m.showAll), m.sp.Tick)
		case "a":
			m.showAll = !m.showAll
			m.loading = true
			return m, tea.Batch(loadProcs(m.showAll), m.sp.Tick)
		}
	}
	return m, nil
}

func (m psModel) View() string {
	if m.loading && len(m.procs) == 0 {
		return "\n  " + m.sp.View() + " reading processes...\n"
	}

	var b strings.Builder
	refreshing := ""
	if m.loading {
		refreshing = "  " + m.sp.View()
	}
	filterTag := ""
	if !m.showAll {
		filterTag = sMuted.Render("  dev only")
	}
	b.WriteString("\n  " + sCyan.Render("🎬 Portflix") +
		sMuted.Render(fmt.Sprintf("  %d process%s", len(m.procs), psPlural(len(m.procs)))) +
		filterTag + refreshing + "\n\n")

	const (
		wpPID  = 7
		wpProc = 12
		wpCPU  = 6
		wpMem  = 10
		wpProj = 18
		wpFW   = 12
		wpUp   = 9
		wpDesc = 30
	)

	h := sColHeader
	hdr := "  " +
		cell(h.Render("PID"), wpPID) + sep +
		cell(h.Render("PROCESS"), wpProc) + sep +
		cell(h.Render("CPU%"), wpCPU) + sep +
		cell(h.Render("MEM"), wpMem) + sep +
		cell(h.Render("PROJECT"), wpProj) + sep +
		cell(h.Render("FRAMEWORK"), wpFW) + sep +
		cell(h.Render("UPTIME"), wpUp) + sep +
		cell(h.Render("WHAT"), wpDesc)
	divLine := "  " + sDivider.Render(strings.Repeat("─",
		wpPID+wpProc+wpCPU+wpMem+wpProj+wpFW+wpUp+wpDesc+lipgloss.Width(sep)*7))
	b.WriteString(hdr + "\n" + divLine + "\n")

	for i, p := range m.procs {
		sel := i == m.cursor
		pidS := fmt.Sprintf("%d", p.PID)
		cpuS := fmt.Sprintf("%.1f", p.CPU)

		var cpuColor lipgloss.Color
		switch {
		case p.CPU > 25:
			cpuColor = cRed
		case p.CPU > 5:
			cpuColor = cYellow
		default:
			cpuColor = cGreen
		}

		cursor := "  "
		if sel {
			cursor = sCyan.Render("▶ ")
		}

		bg := sSelected
		var pidCell, procCell, cpuCell, memCell, projCell, fwCell, upCell, descCell string
		if sel {
			pidCell = cell(bg.Foreground(cMuted).Render(pidS), wpPID)
			procCell = cell(bg.Bold(true).Render(p.ProcessName), wpProc)
			cpuCell = cell(bg.Foreground(cpuColor).Render(cpuS), wpCPU)
			memCell = cell(bg.Foreground(cGreen).Render(coalesce(p.Memory, "—")), wpMem)
			projCell = cell(bg.Foreground(cBlue).Render(coalesce(p.ProjectName, "—")), wpProj)
			fwCell = cell(bg.Render(fwColored(p.Framework)), wpFW)
			upCell = cell(bg.Foreground(cYellow).Render(coalesce(p.Uptime, "—")), wpUp)
			descCell = cell(bg.Foreground(cMuted).Render(coalesce(p.Description, "—")), wpDesc)
		} else {
			pidCell = cell(sMuted.Render(pidS), wpPID)
			procCell = cell(sBold.Render(p.ProcessName), wpProc)
			cpuCell = cell(lipgloss.NewStyle().Foreground(cpuColor).Render(cpuS), wpCPU)
			memCell = cell(sGreen.Render(coalesce(p.Memory, "—")), wpMem)
			projCell = cell(sBlue.Render(coalesce(p.ProjectName, "—")), wpProj)
			fwCell = cell(fwColored(p.Framework), wpFW)
			upCell = cell(sYellow.Render(coalesce(p.Uptime, "—")), wpUp)
			descCell = cell(sMuted.Render(coalesce(p.Description, "—")), wpDesc)
		}

		row := cursor + pidCell + sep + procCell + sep + cpuCell + sep +
			memCell + sep + projCell + sep + fwCell + sep + upCell + sep + descCell
		b.WriteString(row + "\n")
	}

	b.WriteString("\n  " + sMuted.Render("↑↓/jk") + " nav  " +
		sMuted.Render("a") + " all  " +
		sMuted.Render("r") + " refresh  " +
		sMuted.Render("q") + " quit\n")
	return b.String()
}

func psPlural(n int) string {
	if n == 1 {
		return ""
	}
	return "es"
}

// approxKB parses a memory string like "1.2 MB" back to approximate KB.
func approxKB(s string) int {
	var val float64
	var unit string
	fmt.Sscanf(s, "%f %s", &val, &unit)
	switch unit {
	case "GB":
		return int(val * 1048576)
	case "MB":
		return int(val * 1024)
	default:
		return int(val)
	}
}

func RunPS(showAll bool) {
	p := tea.NewProgram(newPSModel(showAll), tea.WithAltScreen())
	p.Run() //nolint
}
