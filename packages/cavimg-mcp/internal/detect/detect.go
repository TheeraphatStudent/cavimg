// Package detect inspects a frontend project directory and reports its package
// manager, framework, TypeScript usage, and source directories.
package detect

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Result is the outcome of detecting a project's stack.
type Result struct {
	PackageManager string            `json:"package_manager"`
	Framework      string            `json:"framework"`
	TypeScript     bool              `json:"typescript"`
	SourceDirs     []string          `json:"source_dirs"`
	Evidence       map[string]string `json:"evidence"`
}

// Run detects the stack of the project rooted at dir.
func Run(dir string) (Result, error) {
	r := Result{SourceDirs: []string{}, Evidence: map[string]string{}}

	var pkg struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
		PackageManager  string            `json:"packageManager"`
	}
	if data, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
		_ = json.Unmarshal(data, &pkg)
	}
	dep := func(name string) bool {
		_, a := pkg.Dependencies[name]
		_, b := pkg.DevDependencies[name]
		return a || b
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(dir, rel))
		return err == nil
	}

	// Package manager: lockfile > packageManager field > default npm.
	switch {
	case exists("pnpm-lock.yaml"):
		r.PackageManager, r.Evidence["package_manager"] = "pnpm", "pnpm-lock.yaml"
	case exists("yarn.lock"):
		r.PackageManager, r.Evidence["package_manager"] = "yarn", "yarn.lock"
	case exists("bun.lockb") || exists("bun.lock"):
		r.PackageManager, r.Evidence["package_manager"] = "bun", "bun lockfile"
	case exists("package-lock.json"):
		r.PackageManager, r.Evidence["package_manager"] = "npm", "package-lock.json"
	case pkg.PackageManager != "":
		r.PackageManager = strings.SplitN(pkg.PackageManager, "@", 2)[0]
		r.Evidence["package_manager"] = "packageManager field"
	default:
		r.PackageManager, r.Evidence["package_manager"] = "npm", "default"
	}

	// Framework: order matters (most specific first).
	switch {
	case dep("next") || exists("next.config.js") || exists("next.config.ts") || exists("next.config.mjs"):
		r.Framework, r.Evidence["framework"] = "Next.js", "next"
	case dep("nuxt"):
		r.Framework, r.Evidence["framework"] = "Nuxt", "nuxt"
	case dep("@sveltejs/kit"):
		r.Framework, r.Evidence["framework"] = "SvelteKit", "@sveltejs/kit"
	case dep("svelte"):
		r.Framework, r.Evidence["framework"] = "Svelte", "svelte"
	case dep("@angular/core"):
		r.Framework, r.Evidence["framework"] = "Angular", "@angular/core"
	case dep("vite") && dep("react"):
		r.Framework, r.Evidence["framework"] = "Vite+React", "vite+react"
	case dep("vite"):
		r.Framework, r.Evidence["framework"] = "Vite", "vite"
	case dep("vue"):
		r.Framework, r.Evidence["framework"] = "Vue", "vue"
	case exists("index.html"):
		r.Framework, r.Evidence["framework"] = "Plain HTML", "index.html"
	default:
		r.Framework, r.Evidence["framework"] = "Unknown", "none"
	}

	// TypeScript.
	switch {
	case exists("tsconfig.json"):
		r.TypeScript, r.Evidence["typescript"] = true, "tsconfig.json"
	case dep("typescript"):
		r.TypeScript, r.Evidence["typescript"] = true, "typescript dependency"
	case hasTSFile(dir):
		r.TypeScript, r.Evidence["typescript"] = true, ".ts/.tsx file"
	}

	// Source directories.
	for _, d := range []string{"src", "app", "pages", "components"} {
		if exists(d) {
			r.SourceDirs = append(r.SourceDirs, d)
		}
	}
	if len(r.SourceDirs) == 0 && r.Framework == "Plain HTML" {
		r.SourceDirs = append(r.SourceDirs, ".")
	}

	return r, nil
}

// hasTSFile walks dir (skipping node_modules) looking for a .ts/.tsx file.
func hasTSFile(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".ts" || ext == ".tsx" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})
	return found
}
