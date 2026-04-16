package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mhrsntrk/portflix/internal/scanner"
)

type watchModel struct {
	events []watchEvent
	known  map[int]bool
	tick   int
}

type watchEvent struct {
	t       time.Time
	kind    string // "new" | "closed"
	port    *scanner.Port
	portNum int
}

type watchTickMsg struct{}
type watchInitMsg struct{ ports []scanner.Port }

func newWatchModel() watchModel {
	return watchModel{known: make(map[int]bool)}
}

func watchInit() tea.Cmd {
	return func() tea.Msg {
		ports, _ := scanner.GetListeningPorts(false)
		return watchInitMsg{ports}
	}
}

func watchPoll(known map[int]bool) tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
		return watchTickMsg{}
	})
}

func (m watchModel) Init() tea.Cmd {
	return watchInit()
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case watchInitMsg:
		for _, p := range msg.ports {
			m.known[p.Port] = true
		}
		return m, watchPoll(m.known)

	case watchTickMsg:
		ports, err := scanner.GetListeningPorts(false)
		if err != nil {
			return m, watchPoll(m.known)
		}
		now := time.Now()
		current := make(map[int]bool)
		for i := range ports {
			p := ports[i]
			current[p.Port] = true
			if !m.known[p.Port] {
				m.events = append(m.events, watchEvent{
					t: now, kind: "new", port: &p,
				})
			}
		}
		for port := range m.known {
			if !current[port] {
				m.events = append(m.events, watchEvent{
					t: now, kind: "closed", portNum: port,
				})
			}
		}
		m.known = current
		m.tick++
		// Keep last 100 events
		if len(m.events) > 100 {
			m.events = m.events[len(m.events)-100:]
		}
		return m, watchPoll(m.known)

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m watchModel) View() string {
	var b strings.Builder
	b.WriteString("\n  " + sCyan.Render("🎬 Portflix") +
		sMuted.Render("  watching for port changes...") + "\n\n")

	if len(m.events) == 0 {
		b.WriteString("  " + sMuted.Render("Waiting for changes...") + "\n")
	} else {
		for _, e := range m.events {
			ts := sMuted.Render(e.t.Format("15:04:05"))
			switch e.kind {
			case "new":
				p := e.port
				fw := ""
				if p.Framework != "" {
					fw = "  " + fwColored(p.Framework)
				}
				proj := ""
				if p.ProjectName != "" {
					proj = sBlue.Render("  [" + p.ProjectName + "]")
				}
				b.WriteString(fmt.Sprintf("  %s  %s  :%s ← %s%s%s\n",
					ts,
					sGreen.Render("▲ NEW   "),
					sBold.Render(fmt.Sprintf("%d", p.Port)),
					sWhite.Render(p.ProcessName),
					proj,
					fw,
				))
			case "closed":
				b.WriteString(fmt.Sprintf("  %s  %s  :%s\n",
					ts,
					sRed.Render("▼ CLOSED"),
					sBold.Render(fmt.Sprintf("%d", e.portNum)),
				))
			}
		}
	}

	b.WriteString("\n  " + sMuted.Render("q") + " quit\n")
	return b.String()
}

func RunWatch() {
	p := tea.NewProgram(newWatchModel(), tea.WithAltScreen())
	p.Run() //nolint
}

// RunWatchInline runs watch without alt screen (streaming output to terminal).
func RunWatchInline() {
	known := make(map[int]bool)
	ports, _ := scanner.GetListeningPorts(false)
	for _, p := range ports {
		known[p.Port] = true
	}

	fmt.Println()
	fmt.Println("  " + sCyan.Render("🎬 Portflix") + sMuted.Render("  watching for port changes..."))
	fmt.Println("  " + sMuted.Render("Press Ctrl+C to stop"))
	fmt.Println()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	quit := make(chan struct{})
	go func() {
		var b [1]byte
		for {
			fmt.Scan(&b)
		}
	}()

	for {
		select {
		case <-ticker.C:
			current, _ := scanner.GetListeningPorts(false)
			now := time.Now()
			cur := make(map[int]bool)
			for _, p := range current {
				cur[p.Port] = true
				if !known[p.Port] {
					fw := ""
					if p.Framework != "" {
						fw = "  " + fwColored(p.Framework)
					}
					proj := ""
					if p.ProjectName != "" {
						proj = sBlue.Render("  [" + p.ProjectName + "]")
					}
					fmt.Printf("  %s  %s  :%s ← %s%s%s\n",
						sMuted.Render(now.Format("15:04:05")),
						sGreen.Render("▲ NEW   "),
						sBold.Render(fmt.Sprintf("%d", p.Port)),
						sWhite.Render(p.ProcessName),
						proj, fw,
					)
				}
			}
			for port := range known {
				if !cur[port] {
					fmt.Printf("  %s  %s  :%s\n",
						sMuted.Render(now.Format("15:04:05")),
						sRed.Render("▼ CLOSED"),
						sBold.Render(fmt.Sprintf("%d", port)),
					)
				}
			}
			known = cur
		case <-quit:
			return
		}
	}
}

func init() {
	_ = strings.Builder{} // ensure import used
}
