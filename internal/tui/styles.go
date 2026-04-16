package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

const asciiLogo = `                  |    _| |_)
 __ \   _ \   __| __| |   | |\ \  /
 |   | (   | |    |   __| | | ` + "`" + `  <
 .__/ \___/ _|   \__|_|  _|_| _/\_\
_|                                  `

func renderBanner() string {
	return sAccent.Render(asciiLogo)
}

var (
	cAccent  = lipgloss.Color("#EF4444")
	cGreen   = lipgloss.Color("#00FF7F")
	cYellow  = lipgloss.Color("#FFD700")
	cRed     = lipgloss.Color("#FF5F5F")
	cMuted   = lipgloss.Color("#585858")
	cBlue    = lipgloss.Color("#5F87FF")
	cMagenta = lipgloss.Color("#D787FF")
	cWhite   = lipgloss.Color("#EEEEEE")
	cSel     = lipgloss.Color("#2C2C2C")
	cOrange  = lipgloss.Color("#DEA55D")
	cCyan    = lipgloss.Color("#00D7FF")

	sMuted   = lipgloss.NewStyle().Foreground(cMuted)
	sGreen   = lipgloss.NewStyle().Foreground(cGreen)
	sYellow  = lipgloss.NewStyle().Foreground(cYellow)
	sRed     = lipgloss.NewStyle().Foreground(cRed)
	sBlue    = lipgloss.NewStyle().Foreground(cBlue)
	sWhite   = lipgloss.NewStyle().Foreground(cWhite)
	sBold    = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sAccent  = lipgloss.NewStyle().Bold(true).Foreground(cAccent)

	sColHeader = lipgloss.NewStyle().Bold(true).Foreground(cWhite)
	sDivider   = lipgloss.NewStyle().Foreground(cMuted)
	sSelected  = lipgloss.NewStyle().Background(cSel)

	sFooter = lipgloss.NewStyle().
		Foreground(cMuted).
		BorderStyle(lipgloss.NormalBorder()).
		BorderTop(true).
		BorderForeground(cMuted)

	frameworkColors = map[string]lipgloss.Color{
		"Next.js": cWhite, "Vite": cYellow, "React": cCyan, "Vue": cGreen,
		"Angular": cRed, "Svelte": lipgloss.Color("#FF3E00"),
		"SvelteKit": lipgloss.Color("#FF3E00"), "Express": cMuted,
		"NestJS": cRed, "Nuxt": cGreen, "Remix": cBlue, "Astro": cMagenta,
		"Django": cGreen, "Flask": cWhite, "FastAPI": cCyan, "Rails": cRed,
		"Gatsby": cMagenta, "Go": cCyan, "Rust": cOrange, "Ruby": cRed,
		"Python": cYellow, "Node.js": cGreen, "Java": cOrange,
		"Docker": cBlue, "PostgreSQL": cBlue, "Redis": cRed,
		"MySQL": cBlue, "MongoDB": cGreen, "nginx": cGreen,
		"LocalStack": cWhite, "Kafka": cWhite,
		"Webpack": cBlue, "esbuild": cYellow, "Parcel": cYellow,
		"Hono": cOrange, "Koa": cWhite, "Fastify": cWhite,
	}
)

func fwColored(fw string) string {
	if fw == "" {
		return sMuted.Render("—")
	}
	c, ok := frameworkColors[fw]
	if !ok {
		return sWhite.Render(fw)
	}
	return lipgloss.NewStyle().Foreground(c).Render(fw)
}

func statusDot(status string) string {
	switch status {
	case "healthy":
		return sGreen.Render("●")
	case "orphaned":
		return sYellow.Render("●")
	case "zombie":
		return sRed.Render("●")
	}
	return sMuted.Render("●")
}

func statusCell(status string) string {
	return statusDot(status) + " " + status
}

// pad pads s to width using display width (ANSI-aware).
func pad(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + spaces(width-w)
}

// trunc truncates s to fit within width (display-width aware, adds ellipsis).
func trunc(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	// approximate: trim runes until it fits
	runes := []rune(s)
	for len(runes) > 0 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func cell(s string, width int) string {
	return pad(trunc(s, width), width)
}

func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = ' '
	}
	return string(b)
}

func coalesce(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// pulseFrames cycles a block from near-invisible to full accent color and back,
// simulating a breathing/opacity effect for background activity indicators.
var pulseFrames = []lipgloss.Color{
	"#3a0f0f", "#6b1a1a", "#9e2a2a", "#c93838", "#EF4444",
	"#c93838", "#9e2a2a", "#6b1a1a",
}

func pulseChar(frame int) string {
	c := pulseFrames[frame%len(pulseFrames)]
	return lipgloss.NewStyle().Foreground(c).Render("■")
}

// renderHints renders keyboard hints in priority order, stopping before they
// would exceed the available width. Each hint is a [key, label] pair.
func renderHints(width int, hints [][2]string) string {
	avail := width - 2 // account for "  " indent
	if avail <= 0 {
		avail = 999
	}
	var parts []string
	for _, h := range hints {
		part := sMuted.Render(h[0]) + " " + h[1]
		candidate := strings.Join(append(parts, part), "  ")
		if lipgloss.Width(candidate) > avail && len(parts) > 0 {
			break
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "  ")
}
