package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/mhrsntrk/portflix/internal/scanner"
	"github.com/mhrsntrk/portflix/internal/tui"
	"github.com/spf13/cobra"
)

var (
	cyan   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7FF"))
	green  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF7F"))
	yellow = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	red    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F"))
	muted  = lipgloss.NewStyle().Foreground(lipgloss.Color("#585858"))
	bold   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#EEEEEE"))
	blue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#5F87FF"))
)

func main() {
	var showAll bool

	root := &cobra.Command{
		Use:   "ports [port]",
		Short: "🎬 Portflix — stream your ports",
		Long:  "Portflix: a beautiful TUI for seeing what's running on your ports.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				port, err := strconv.Atoi(args[0])
				if err != nil {
					return fmt.Errorf("not a valid port number: %s", args[0])
				}
				return runInspect(port)
			}
			logsPort := tui.RunPorts(showAll)
			if logsPort != 0 {
				return runLogsForPort(logsPort, false)
			}
			return nil
		},
	}
	root.Flags().BoolVarP(&showAll, "all", "a", false, "Show all ports, not just dev")

	psCmd := &cobra.Command{
		Use:   "ps",
		Short: "Show all running dev processes",
		RunE: func(cmd *cobra.Command, args []string) error {
			tui.RunPS(showAll)
			return nil
		},
	}
	psCmd.Flags().BoolVarP(&showAll, "all", "a", false, "Show all processes")

	killCmd := &cobra.Command{
		Use:   "kill [-f] <port|pid|range> [...]",
		Short: "Kill process(es) by port, PID, or range",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runKill,
	}
	var force bool
	killCmd.Flags().BoolVarP(&force, "force", "f", false, "Force kill (SIGKILL)")

	logsCmd := &cobra.Command{
		Use:   "logs <port|pid>",
		Short: "Tail log output for a process on a port",
		Args:  cobra.ExactArgs(1),
		RunE:  runLogs,
	}
	var follow bool
	logsCmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow (stream new lines)")

	watchCmd := &cobra.Command{
		Use:   "watch",
		Short: "Monitor port changes in real-time",
		RunE: func(cmd *cobra.Command, args []string) error {
			tui.RunWatch()
			return nil
		},
	}

	cleanCmd := &cobra.Command{
		Use:   "clean",
		Short: "Kill orphaned/zombie dev server processes",
		RunE:  runClean,
	}

	root.AddCommand(psCmd, killCmd, logsCmd, watchCmd, cleanCmd)

	// 'portflix' is a second binary name that resolves to inspect-a-port
	portflixCmd := &cobra.Command{
		Use:    "portflix [port]",
		Hidden: true,
		Args:   cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				port, err := strconv.Atoi(args[0])
				if err == nil {
					return runInspect(port)
				}
			}
			logsPort := tui.RunPorts(false)
			if logsPort != 0 {
				return runLogsForPort(logsPort, false)
			}
			return nil
		},
	}
	root.AddCommand(portflixCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func runInspect(port int) error {
	fmt.Println()
	fmt.Println("  " + cyan.Render("🎬 Portflix") + muted.Render(fmt.Sprintf("  inspecting :%d", port)))
	fmt.Println()

	p, err := scanner.ResolvePort(port)
	if err != nil || p == nil {
		fmt.Println("  " + red.Render(fmt.Sprintf("No process found on :%d", port)))
		fmt.Println()
		return nil
	}

	row := func(label, value string) {
		fmt.Printf("  %s%s\n", muted.Render(padStr(label, 16)), value)
	}

	fmt.Println("  " + bold.Render(fmt.Sprintf(":%d", p.Port)))
	fmt.Println("  " + muted.Render(strings.Repeat("─", 40)))
	fmt.Println()
	row("Process", bold.Render(p.ProcessName))
	row("PID", muted.Render(strconv.Itoa(p.PID)))
	row("Status", statusStr(string(p.Status)))
	row("Framework", fwStr(p.Framework))
	row("Memory", green.Render(strDefault(p.Memory, "—")))
	row("Uptime", yellow.Render(strDefault(p.Uptime, "—")))
	if !p.StartTime.IsZero() {
		row("Started", muted.Render(p.StartTime.Format("Jan 2 15:04:05")))
	}
	fmt.Println()
	fmt.Println("  " + cyan.Render("Location"))
	fmt.Println("  " + muted.Render(strings.Repeat("─", 40)))
	fmt.Println()
	row("Directory", blue.Render(strDefault(p.CWD, "—")))
	row("Project", bold.Render(strDefault(p.ProjectName, "—")))
	if p.GitBranch != "" {
		row("Branch", lipgloss.NewStyle().Foreground(lipgloss.Color("#D787FF")).Render(p.GitBranch))
	}
	fmt.Println()
	fmt.Print("  " + yellow.Render(fmt.Sprintf("Kill :%d? [y/N] ", port)))

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "y" {
		if scanner.KillProcess(p.PID, false) {
			fmt.Println("  " + green.Render(fmt.Sprintf("✓ Killed PID %d", p.PID)))
		} else {
			fmt.Println("  " + red.Render(fmt.Sprintf("✕ Failed — try: sudo kill -9 %d", p.PID)))
		}
	}
	fmt.Println()
	return nil
}

func runKill(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	sig := "SIGTERM"
	if force {
		sig = "SIGKILL"
	}

	// Expand ranges
	var targets []string
	var rangeGroups []struct{ start, end int }

	for _, arg := range args {
		if idx := strings.Index(arg, "-"); idx > 0 {
			start, e1 := strconv.Atoi(arg[:idx])
			end, e2 := strconv.Atoi(arg[idx+1:])
			if e1 == nil && e2 == nil && end > start && end-start <= 1000 &&
				start >= 1 && end <= 65535 {
				rangeGroups = append(rangeGroups, struct{ start, end int }{start, end})
				for p := start; p <= end; p++ {
					targets = append(targets, strconv.Itoa(p))
				}
				continue
			}
		}
		targets = append(targets, arg)
	}

	fmt.Println()
	killed, failed, empty := 0, 0, 0
	inRange := func(n int) bool {
		for _, r := range rangeGroups {
			if n >= r.start && n <= r.end {
				return true
			}
		}
		return false
	}

	for _, t := range targets {
		n, err := strconv.Atoi(t)
		if err != nil {
			fmt.Println("  " + red.Render(fmt.Sprintf("✕ \"%s\" is not a valid port/PID", t)))
			failed++
			continue
		}

		p, viaPort := scanner.ResolveTarget(n)
		if p == nil {
			if inRange(n) {
				empty++
				continue
			}
			if n <= 65535 {
				fmt.Println("  " + red.Render(fmt.Sprintf("✕ No listener on :%d and no process with PID %d", n, n)))
			} else {
				fmt.Println("  " + red.Render(fmt.Sprintf("✕ No process with PID %d", n)))
			}
			failed++
			continue
		}

		label := fmt.Sprintf("PID %d", p.PID)
		if viaPort {
			label = fmt.Sprintf(":%d — %s (PID %d)", n, p.ProcessName, p.PID)
		}
		fmt.Println("  " + muted.Render(fmt.Sprintf("Killing %s", label)))
		if scanner.KillProcess(p.PID, force) {
			fmt.Println("  " + green.Render(fmt.Sprintf("✓ Sent %s to %s", sig, label)))
			killed++
		} else {
			fmt.Println("  " + red.Render(fmt.Sprintf("✕ Failed — try: sudo kill%s %d",
				map[bool]string{true: " -9", false: ""}[force], p.PID)))
			failed++
		}
	}

	if len(rangeGroups) > 0 {
		var parts []string
		if killed > 0 {
			parts = append(parts, green.Render(fmt.Sprintf("%d killed", killed)))
		}
		if empty > 0 {
			parts = append(parts, muted.Render(fmt.Sprintf("%d empty", empty)))
		}
		if failed > 0 {
			parts = append(parts, red.Render("some failed"))
		}
		fmt.Println("  " + muted.Render("Range summary: ") + strings.Join(parts, muted.Render(", ")))
	}
	fmt.Println()
	if failed > 0 {
		os.Exit(1)
	}
	return nil
}

func runLogs(cmd *cobra.Command, args []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	n, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("not a valid port/PID: %s", args[0])
	}
	return runLogsForPort(n, follow)
}

func runLogsForPort(n int, follow bool) error {
	p, _ := scanner.ResolveTarget(n)
	if p == nil {
		fmt.Println()
		fmt.Println("  " + red.Render(fmt.Sprintf("✕ No process found for %d", n)))
		fmt.Println()
		return nil
	}
	port := 0
	if n <= 65535 {
		port = n
	}
	tui.RunLogs(port, p.PID, p.ProcessName, follow)
	return nil
}

func runClean(cmd *cobra.Command, args []string) error {
	orphaned, err := scanner.FindOrphanedPorts()
	if err != nil {
		return err
	}
	fmt.Println()
	if len(orphaned) == 0 {
		fmt.Println("  " + green.Render("✓ No orphaned or zombie processes found. All clean!"))
		fmt.Println()
		return nil
	}

	fmt.Println("  " + yellow.Render(fmt.Sprintf("Found %d orphaned/zombie process%s:",
		len(orphaned), map[bool]string{true: "", false: "es"}[len(orphaned) == 1])))
	fmt.Println()
	for _, p := range orphaned {
		fmt.Printf("  %s  :%s — %s %s\n",
			muted.Render("•"),
			bold.Render(strconv.Itoa(p.Port)),
			p.ProcessName,
			muted.Render(fmt.Sprintf("(PID %d)", p.PID)),
		)
	}
	fmt.Println()
	fmt.Print("  " + yellow.Render("Kill all? [y/N] "))

	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	if strings.TrimSpace(strings.ToLower(answer)) != "y" {
		fmt.Println(muted.Render("\n  Aborted."))
		fmt.Println()
		return nil
	}

	killed, failed := 0, 0
	for _, p := range orphaned {
		if scanner.KillProcess(p.PID, false) {
			fmt.Println("  " + green.Render(fmt.Sprintf("✓ Killed :%d (PID %d)", p.Port, p.PID)))
			killed++
		} else {
			fmt.Println("  " + red.Render(fmt.Sprintf("✕ Failed :%d — try: sudo kill -9 %d", p.Port, p.PID)))
			failed++
		}
	}
	fmt.Println()
	if killed > 0 {
		fmt.Println("  " + green.Render(fmt.Sprintf("Cleaned %d process%s.", killed,
			map[bool]string{true: "", false: "es"}[killed == 1])))
	}
	if failed > 0 {
		fmt.Println("  " + red.Render(fmt.Sprintf("Failed to clean %d process%s.",
			failed, map[bool]string{true: "", false: "es"}[failed == 1])))
	}
	fmt.Println()
	return nil
}

// helpers

func padStr(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func strDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func statusStr(status string) string {
	switch status {
	case "healthy":
		return green.Render("● ") + "healthy"
	case "orphaned":
		return yellow.Render("● ") + "orphaned"
	case "zombie":
		return red.Render("● ") + "zombie"
	}
	return muted.Render("● unknown")
}

func fwStr(fw string) string {
	if fw == "" {
		return muted.Render("—")
	}
	return fw
}
