package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

func IsDevProcess(name, cmd string) bool {
	lower := strings.ToLower(name)
	cmdLower := strings.ToLower(cmd)

	systemApps := []string{
		"spotify", "raycast", "tableplus", "postman", "linear", "cursor",
		"controlce", "rapportd", "superhuma", "setappage", "slack", "discord",
		"firefox", "chrome", "google", "safari", "figma", "notion", "zoom",
		"teams", "code", "iterm2", "warp", "arc", "loginwindow", "windowserver",
		"systemuise", "kernel_task", "launchd", "mdworker", "mds_stores",
		"cfprefsd", "coreaudio", "corebrightne", "airportd", "bluetoothd",
		"sharingd", "usernoted", "notificationc", "cloudd",
	}
	for _, app := range systemApps {
		if strings.HasPrefix(lower, app) {
			return false
		}
	}

	devNames := map[string]bool{
		"node": true, "python": true, "python3": true, "ruby": true,
		"java": true, "go": true, "cargo": true, "deno": true, "bun": true,
		"php": true, "uvicorn": true, "gunicorn": true, "flask": true,
		"rails": true, "npm": true, "npx": true, "yarn": true, "pnpm": true,
		"tsc": true, "tsx": true, "esbuild": true, "rollup": true,
		"turbo": true, "nx": true, "jest": true, "vitest": true,
		"pytest": true, "cypress": true, "playwright": true,
		"rustc": true, "dotnet": true, "gradle": true, "mvn": true,
		"mix": true, "elixir": true,
	}
	if devNames[lower] {
		return true
	}
	if strings.HasPrefix(lower, "com.docke") || lower == "docker" || lower == "docker-sandbox" {
		return true
	}
	for _, ind := range []string{
		"node", "next", "vite", "nuxt", "webpack", "remix", "astro",
		"gulp", "ng serve", "gatsby", "flask", "django", "manage.py",
		"uvicorn", "rails", "cargo",
	} {
		if strings.Contains(cmdLower, ind) {
			return true
		}
	}
	return false
}

func DetectFramework(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "package.json"))
	if err == nil {
		var pkg struct {
			Dependencies    map[string]string `json:"dependencies"`
			DevDependencies map[string]string `json:"devDependencies"`
		}
		if json.Unmarshal(data, &pkg) == nil {
			all := make(map[string]bool)
			for k := range pkg.Dependencies {
				all[k] = true
			}
			for k := range pkg.DevDependencies {
				all[k] = true
			}
			switch {
			case all["next"]:
				return "Next.js"
			case all["nuxt"] || all["nuxt3"]:
				return "Nuxt"
			case all["@sveltejs/kit"]:
				return "SvelteKit"
			case all["svelte"]:
				return "Svelte"
			case all["@remix-run/react"] || all["remix"]:
				return "Remix"
			case all["astro"]:
				return "Astro"
			case all["vite"]:
				return "Vite"
			case all["@angular/core"]:
				return "Angular"
			case all["vue"]:
				return "Vue"
			case all["react"]:
				return "React"
			case all["express"]:
				return "Express"
			case all["fastify"]:
				return "Fastify"
			case all["hono"]:
				return "Hono"
			case all["koa"]:
				return "Koa"
			case all["@nestjs/core"]:
				return "NestJS"
			case all["gatsby"]:
				return "Gatsby"
			case all["webpack-dev-server"]:
				return "Webpack"
			case all["esbuild"]:
				return "esbuild"
			case all["parcel"]:
				return "Parcel"
			}
		}
	}
	for _, check := range [][2]string{
		{"vite.config.ts", "Vite"}, {"vite.config.js", "Vite"},
		{"next.config.js", "Next.js"}, {"next.config.mjs", "Next.js"},
		{"angular.json", "Angular"}, {"Cargo.toml", "Rust"},
		{"go.mod", "Go"}, {"manage.py", "Django"}, {"Gemfile", "Ruby"},
	} {
		if _, err := os.Stat(filepath.Join(root, check[0])); err == nil {
			return check[1]
		}
	}
	return ""
}

func DetectFrameworkFromCommand(cmd, name string) string {
	if cmd == "" {
		return detectFrameworkFromName(name)
	}
	lower := strings.ToLower(cmd)
	switch {
	case strings.Contains(lower, "next"):
		return "Next.js"
	case strings.Contains(lower, "vite"):
		return "Vite"
	case strings.Contains(lower, "nuxt"):
		return "Nuxt"
	case strings.Contains(lower, "angular") || strings.Contains(lower, "ng serve"):
		return "Angular"
	case strings.Contains(lower, "webpack"):
		return "Webpack"
	case strings.Contains(lower, "remix"):
		return "Remix"
	case strings.Contains(lower, "astro"):
		return "Astro"
	case strings.Contains(lower, "gatsby"):
		return "Gatsby"
	case strings.Contains(lower, "flask"):
		return "Flask"
	case strings.Contains(lower, "django") || strings.Contains(lower, "manage.py"):
		return "Django"
	case strings.Contains(lower, "uvicorn"):
		return "FastAPI"
	case strings.Contains(lower, "rails"):
		return "Rails"
	case strings.Contains(lower, "cargo") || strings.Contains(lower, "rustc"):
		return "Rust"
	}
	return detectFrameworkFromName(name)
}

func DetectFrameworkFromImage(image string) string {
	img := strings.ToLower(image)
	switch {
	case strings.Contains(img, "postgres"):
		return "PostgreSQL"
	case strings.Contains(img, "redis"):
		return "Redis"
	case strings.Contains(img, "mysql") || strings.Contains(img, "mariadb"):
		return "MySQL"
	case strings.Contains(img, "mongo"):
		return "MongoDB"
	case strings.Contains(img, "nginx"):
		return "nginx"
	case strings.Contains(img, "localstack"):
		return "LocalStack"
	case strings.Contains(img, "rabbitmq"):
		return "RabbitMQ"
	case strings.Contains(img, "kafka"):
		return "Kafka"
	case strings.Contains(img, "elasticsearch") || strings.Contains(img, "opensearch"):
		return "Elasticsearch"
	case strings.Contains(img, "minio"):
		return "MinIO"
	}
	return "Docker"
}

func detectFrameworkFromName(name string) string {
	switch strings.ToLower(name) {
	case "node":
		return "Node.js"
	case "python", "python3":
		return "Python"
	case "ruby":
		return "Ruby"
	case "java":
		return "Java"
	case "go":
		return "Go"
	}
	return ""
}
