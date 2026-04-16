package tui

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mhrsntrk/portflix/internal/scanner"
)

type logsModel struct {
	port      int
	pid       int
	procName  string
	logPath   string
	fdLabel   string
	vp        viewport.Model
	sp        spinner.Model
	loading   bool
	following bool
	lines     []string
	err       string
	width     int
	height    int
}

type logsReadyMsg struct {
	path    string
	fdLabel string
	content string
}
type logsLineMsg struct{ line string }
type logsErrMsg struct{ msg string }
type logsFallbackMsg struct{ cmd string; content string }

func newLogsModel(port, pid int, procName string, follow bool) logsModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(cCyan)
	vp := viewport.New(0, 0)
	return logsModel{
		port: port, pid: pid, procName: procName,
		following: follow, loading: true, sp: sp, vp: vp,
	}
}

func resolveLogFiles(pid int, follow bool, numLines int) tea.Cmd {
	return func() tea.Msg {
		files := scanner.GetProcessLogFiles(pid)
		if len(files) == 0 {
			// Fall back to system log
			sysCmd := scanner.GetSystemLogCommand(pid, follow)
			out := runHeadLines(sysCmd, numLines)
			return logsFallbackMsg{cmd: sysCmd, content: out}
		}
		// Use first (highest-priority) file
		f := files[0]
		content := tailLines(f.Path, numLines)
		return logsReadyMsg{path: f.Path, fdLabel: f.FD, content: content}
	}
}

func tailLines(path string, n int) string {
	out, err := exec.Command("tail", "-n", fmt.Sprintf("%d", n), path).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func runHeadLines(cmd string, n int) string {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}
	out, err := exec.Command(parts[0], parts[1:]...).Output()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func followFile(path string) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(path)
		if err != nil {
			return logsErrMsg{fmt.Sprintf("cannot open %s: %v", path, err)}
		}
		f.Seek(0, io.SeekEnd)
		reader := bufio.NewReader(f)

		// Poll for new content every 200ms
		for {
			line, err := reader.ReadString('\n')
			if err == nil && line != "" {
				return logsLineMsg{strings.TrimRight(line, "\n")}
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func (m logsModel) Init() tea.Cmd {
	return tea.Batch(resolveLogFiles(m.pid, m.following, 50), m.sp.Tick)
}

func (m logsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var vpCmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		headerH := 4
		footerH := 2
		m.vp.Width = m.width - 4
		m.vp.Height = m.height - headerH - footerH

	case spinner.TickMsg:
		if m.loading {
			var cmd tea.Cmd
			m.sp, cmd = m.sp.Update(msg)
			return m, cmd
		}

	case logsReadyMsg:
		m.loading = false
		m.logPath = msg.path
		m.fdLabel = msg.fdLabel
		m.lines = strings.Split(strings.TrimSpace(msg.content), "\n")
		m.vp.SetContent(strings.Join(m.lines, "\n"))
		m.vp.GotoBottom()
		if m.following {
			return m, followFile(m.logPath)
		}

	case logsFallbackMsg:
		m.loading = false
		m.logPath = msg.cmd
		m.fdLabel = "system"
		m.lines = strings.Split(strings.TrimSpace(msg.content), "\n")
		m.vp.SetContent(strings.Join(m.lines, "\n"))
		m.vp.GotoBottom()

	case logsLineMsg:
		m.lines = append(m.lines, msg.line)
		m.vp.SetContent(strings.Join(m.lines, "\n"))
		m.vp.GotoBottom()
		return m, followFile(m.logPath)

	case logsErrMsg:
		m.err = msg.msg
		m.loading = false

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.vp, vpCmd = m.vp.Update(msg)
	return m, vpCmd
}

func (m logsModel) View() string {
	if m.loading {
		return "\n  " + m.sp.View() + " finding log files...\n"
	}

	portLabel := fmt.Sprintf(":%d", m.port)
	if m.port == 0 {
		portLabel = fmt.Sprintf("PID %d", m.pid)
	}

	followStr := ""
	if m.following {
		followStr = sGreen.Render("  ● following")
	}

	header := renderBanner() + "\n" +
		"  " + sMuted.Render(fmt.Sprintf("logs for %s (%s, PID %d)", portLabel, m.procName, m.pid)) +
		followStr + "\n"

	fileInfo := ""
	if m.logPath != "" {
		fdStyle := sGreen
		if m.fdLabel == "stderr" {
			fdStyle = sYellow
		}
		fileInfo = "  " + fdStyle.Render("▸ ") + sMuted.Render(m.logPath) + "\n"
	}
	if m.err != "" {
		fileInfo = "  " + sRed.Render(m.err) + "\n"
	}

	content := lipgloss.NewStyle().
		PaddingLeft(2).
		Foreground(cWhite).
		Render(m.vp.View())

	footer := sMuted.Render("  ↑↓ scroll  q quit")
	if m.following {
		footer = sMuted.Render("  auto-scrolling  q quit")
	}

	return header + fileInfo + "\n" + content + "\n" + footer + "\n"
}

func RunLogs(port, pid int, procName string, follow bool) {
	m := newLogsModel(port, pid, procName, follow)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	p.Run() //nolint
}
