package scanner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Status string

const (
	StatusHealthy  Status = "healthy"
	StatusOrphaned Status = "orphaned"
	StatusZombie   Status = "zombie"
)

type Port struct {
	Port        int
	PID         int
	ProcessName string
	RawName     string
	Command     string
	CWD         string
	ProjectName string
	Framework   string
	Uptime      string
	StartTime   time.Time
	Status      Status
	Memory      string
	GitBranch   string
	PPID        int
}

type Process struct {
	PID         int
	ProcessName string
	Command     string
	Description string
	CPU         float64
	Memory      string
	Framework   string
	Uptime      string
	ProjectName string
	CWD         string
}

type LogFile struct {
	Path     string
	FD       string
	FileType string
	Priority int
}

type dockerContainer struct{ name, image string }

type psInfo struct {
	ppid    int
	stat    string
	rssKB   int
	elapsed time.Duration
	command string
}

func batchProcessInfo(pids []int) map[int]*psInfo {
	result := make(map[int]*psInfo)
	if len(pids) == 0 {
		return result
	}
	strs := make([]string, len(pids))
	for i, p := range pids {
		strs[i] = strconv.Itoa(p)
	}
	out, err := exec.Command("ps",
		"-p", strings.Join(strs, ","),
		"-o", "pid=,ppid=,stat=,rss=,etime=,command=",
	).Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			continue
		}
		ppid, _ := strconv.Atoi(fields[1])
		rss, _ := strconv.Atoi(fields[3])
		cmd := ""
		if len(fields) > 5 {
			cmd = strings.Join(fields[5:], " ")
		}
		result[pid] = &psInfo{
			ppid:    ppid,
			stat:    fields[2],
			rssKB:   rss,
			elapsed: parseEtime(fields[4]),
			command: cmd,
		}
	}
	return result
}

func batchCWD(pids []int) map[int]string {
	result := make(map[int]string)
	if len(pids) == 0 {
		return result
	}
	strs := make([]string, len(pids))
	for i, p := range pids {
		strs[i] = strconv.Itoa(p)
	}
	out, err := exec.Command("lsof", "-a", "-d", "cwd",
		"-p", strings.Join(strs, ",")).Output()
	if err != nil {
		return result
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		path := strings.Join(fields[8:], " ")
		if strings.HasPrefix(path, "/") {
			result[pid] = path
		}
	}
	return result
}

func batchDockerInfo() map[int]dockerContainer {
	result := make(map[int]dockerContainer)
	out, err := exec.Command("docker", "ps",
		"--format", "{{.Ports}}\t{{.Names}}\t{{.Image}}").Output()
	if err != nil {
		return result
	}
	re := regexp.MustCompile(`(?:\d+\.\d+\.\d+\.\d+|::):(\d+)->`)
	seen := map[int]bool{}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}
		for _, m := range re.FindAllStringSubmatch(parts[0], -1) {
			port, _ := strconv.Atoi(m[1])
			if !seen[port] {
				seen[port] = true
				result[port] = dockerContainer{parts[1], parts[2]}
			}
		}
	}
	return result
}

var portRe = regexp.MustCompile(`:(\d+)$`)

func GetListeningPorts(detailed bool) ([]Port, error) {
	out, err := exec.Command("lsof", "-iTCP", "-sTCP:LISTEN", "-P", "-n").Output()
	if err != nil && len(out) == 0 {
		return nil, nil
	}

	type raw struct{ port, pid int; name string }
	var entries []raw
	seen := map[int]bool{}

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n")[1:] {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		m := portRe.FindStringSubmatch(fields[8])
		if m == nil {
			continue
		}
		port, _ := strconv.Atoi(m[1])
		if seen[port] {
			continue
		}
		seen[port] = true
		entries = append(entries, raw{port, pid, fields[0]})
	}
	if len(entries) == 0 {
		return nil, nil
	}

	pidSet := map[int]bool{}
	for _, e := range entries {
		pidSet[e.pid] = true
	}
	pids := make([]int, 0, len(pidSet))
	for pid := range pidSet {
		pids = append(pids, pid)
	}

	psMap := batchProcessInfo(pids)
	cwdMap := batchCWD(pids)

	dockerMap := batchDockerInfo()

	var ports []Port
	for _, e := range entries {
		ps := psMap[e.pid]
		p := Port{
			Port:        e.port,
			PID:         e.pid,
			ProcessName: e.name,
			RawName:     e.name,
			Status:      StatusHealthy,
		}
		if ps != nil {
			p.Command = ps.command
			p.PPID = ps.ppid
			if ps.rssKB > 0 {
				p.Memory = formatMemory(ps.rssKB)
			}
			if ps.elapsed > 0 {
				p.Uptime = formatDuration(ps.elapsed)
				p.StartTime = time.Now().Add(-ps.elapsed)
			}
			if strings.Contains(ps.stat, "Z") {
				p.Status = StatusZombie
			} else if ps.ppid == 1 && IsDevProcess(e.name, ps.command) {
				p.Status = StatusOrphaned
			}
			p.Framework = DetectFrameworkFromCommand(ps.command, e.name)
		}
		if dc, ok := dockerMap[e.port]; ok {
			p.ProjectName = dc.name
			p.Framework = DetectFrameworkFromImage(dc.image)
			p.ProcessName = "docker"
		} else if cwd := cwdMap[e.pid]; cwd != "" {
			root := findProjectRoot(cwd)
			p.CWD = root
			p.ProjectName = filepath.Base(root)
			if p.Framework == "" {
				p.Framework = DetectFramework(root)
			}
			if detailed {
				p.GitBranch = gitBranch(root)
			}
		}
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool { return ports[i].Port < ports[j].Port })
	return ports, nil
}

func GetAllProcesses() ([]Process, error) {
	out, err := exec.Command("ps", "-eo", "pid=,pcpu=,rss=,etime=,command=").Output()
	if err != nil {
		return nil, err
	}
	self := os.Getpid()
	seen := map[int]bool{}
	var entries []Process

	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid <= 1 || pid == self || seen[pid] {
			continue
		}
		seen[pid] = true
		cpu, _ := strconv.ParseFloat(fields[1], 64)
		rss, _ := strconv.Atoi(fields[2])
		elapsed := parseEtime(fields[3])
		cmd := ""
		if len(fields) > 4 {
			cmd = strings.Join(fields[4:], " ")
		}
		name := filepath.Base(strings.Fields(cmd)[0])

		p := Process{
			PID:         pid,
			ProcessName: name,
			Command:     cmd,
			Description: summarizeCommand(cmd, name),
			CPU:         cpu,
			Framework:   DetectFrameworkFromCommand(cmd, name),
		}
		if rss > 0 {
			p.Memory = formatMemory(rss)
		}
		if elapsed > 0 {
			p.Uptime = formatDuration(elapsed)
		}
		entries = append(entries, p)
	}

	pids := make([]int, len(entries))
	for i, e := range entries {
		pids[i] = e.PID
	}
	cwdMap := batchCWD(pids)
	for i := range entries {
		if cwd := cwdMap[entries[i].PID]; cwd != "" {
			root := findProjectRoot(cwd)
			entries[i].CWD = root
			entries[i].ProjectName = filepath.Base(root)
			if entries[i].Framework == "" {
				entries[i].Framework = DetectFramework(root)
			}
		}
	}
	return entries, nil
}

func FindOrphanedPorts() ([]Port, error) {
	ports, err := GetListeningPorts(false)
	if err != nil {
		return nil, err
	}
	var out []Port
	for _, p := range ports {
		if p.Status == StatusOrphaned || p.Status == StatusZombie {
			out = append(out, p)
		}
	}
	return out, nil
}

func KillProcess(pid int, force bool) bool {
	sig := "-TERM"
	if force {
		sig = "-KILL"
	}
	return exec.Command("kill", sig, strconv.Itoa(pid)).Run() == nil
}

func PIDExists(pid int) bool {
	return exec.Command("kill", "-0", strconv.Itoa(pid)).Run() == nil
}

func ResolvePort(port int) (*Port, error) {
	ports, err := GetListeningPorts(true)
	if err != nil {
		return nil, err
	}
	for i := range ports {
		if ports[i].Port == port {
			return &ports[i], nil
		}
	}
	return nil, nil
}

func FormatMemoryKB(kb int) string { return formatMemory(kb) }

func ResolveTarget(n int) (*Port, bool) {
	if n <= 65535 {
		ports, _ := GetListeningPorts(false)
		for i := range ports {
			if ports[i].Port == n {
				return &ports[i], true // via port
			}
		}
	}
	if PIDExists(n) {
		return &Port{PID: n, ProcessName: "unknown"}, false // via pid
	}
	return nil, false
}

func GetProcessLogFiles(pid int) []LogFile {
	var files []LogFile
	out, err := exec.Command("lsof", "-p", strconv.Itoa(pid)).Output()
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n")[1:] {
			cols := strings.Fields(line)
			if len(cols) < 9 {
				continue
			}
			fd, typ, name := cols[3], cols[4], strings.Join(cols[8:], " ")
			if (fd == "1w" || fd == "2w") && typ == "REG" {
				fdName := "stdout"
				if fd == "2w" {
					fdName = "stderr"
				}
				files = append(files, LogFile{name, fdName, "redirect", 1})
				continue
			}
			if typ == "REG" && strings.HasSuffix(fd, "w") && isLogLikePath(name) {
				files = append(files, LogFile{name, "file", "logfile", 2})
			}
		}
	}
	if cwd := getProcessCWD(pid); cwd != "" {
		for _, rel := range []string{
			".next/server.log", "logs/development.log", "log/development.log",
			"storage/logs/laravel.log",
		} {
			full := filepath.Join(cwd, rel)
			if _, err := os.Stat(full); err == nil {
				files = append(files, LogFile{full, "file", "framework", 3})
			}
		}
	}
	// deduplicate
	pathSeen := map[string]bool{}
	unique := files[:0]
	for _, f := range files {
		if !pathSeen[f.Path] {
			pathSeen[f.Path] = true
			unique = append(unique, f)
		}
	}
	sort.Slice(unique, func(i, j int) bool { return unique[i].Priority < unique[j].Priority })
	return unique
}

func GetSystemLogCommand(pid int, follow bool) string {
	if follow {
		return fmt.Sprintf("log stream --predicate 'processID == %d' --style compact", pid)
	}
	return fmt.Sprintf("log show --predicate 'processID == %d' --style compact --last 1m", pid)
}

func getProcessCWD(pid int) string {
	out, err := exec.Command("lsof", "-p", strconv.Itoa(pid), "-d", "cwd", "-Fn").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "n") {
			return strings.TrimPrefix(strings.TrimSpace(line), "n")
		}
	}
	return ""
}

func isLogLikePath(name string) bool {
	l := strings.ToLower(name)
	return strings.HasSuffix(l, ".log") ||
		strings.Contains(l, "/log/") ||
		strings.Contains(l, "/logs/") ||
		strings.Contains(l, "/tmp/") ||
		strings.Contains(l, "stdout") ||
		strings.Contains(l, "stderr")
}

func findProjectRoot(dir string) string {
	markers := []string{"package.json", "Cargo.toml", "go.mod", "pyproject.toml", "Gemfile", "pom.xml"}
	cur := dir
	for depth := 0; depth < 15; depth++ {
		for _, m := range markers {
			if _, err := os.Stat(filepath.Join(cur, m)); err == nil {
				return cur
			}
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dir
}

func gitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func summarizeCommand(cmd, name string) string {
	parts := strings.Fields(cmd)
	var meaningful []string
	for i, p := range parts {
		if i == 0 || strings.HasPrefix(p, "-") {
			continue
		}
		if strings.Contains(p, "/") {
			meaningful = append(meaningful, filepath.Base(p))
		} else {
			meaningful = append(meaningful, p)
		}
		if len(meaningful) >= 3 {
			break
		}
	}
	if len(meaningful) > 0 {
		return strings.Join(meaningful, " ")
	}
	return name
}

func parseEtime(s string) time.Duration {
	s = strings.TrimSpace(s)
	var days, hours, mins, secs int
	if idx := strings.Index(s, "-"); idx != -1 {
		days, _ = strconv.Atoi(s[:idx])
		s = s[idx+1:]
	}
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3:
		hours, _ = strconv.Atoi(parts[0])
		mins, _ = strconv.Atoi(parts[1])
		secs, _ = strconv.Atoi(parts[2])
	case 2:
		mins, _ = strconv.Atoi(parts[0])
		secs, _ = strconv.Atoi(parts[1])
	case 1:
		secs, _ = strconv.Atoi(parts[0])
	}
	return time.Duration(days)*24*time.Hour +
		time.Duration(hours)*time.Hour +
		time.Duration(mins)*time.Minute +
		time.Duration(secs)*time.Second
}

func formatDuration(d time.Duration) string {
	t := int(d.Seconds())
	days := t / 86400
	hours := (t % 86400) / 3600
	mins := (t % 3600) / 60
	secs := t % 60
	switch {
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, mins)
	case mins > 0:
		return fmt.Sprintf("%dm %ds", mins, secs)
	default:
		return fmt.Sprintf("%ds", secs)
	}
}

func formatMemory(rssKB int) string {
	switch {
	case rssKB > 1048576:
		return fmt.Sprintf("%.1f GB", float64(rssKB)/1048576)
	case rssKB > 1024:
		return fmt.Sprintf("%.1f MB", float64(rssKB)/1024)
	default:
		return fmt.Sprintf("%d KB", rssKB)
	}
}
