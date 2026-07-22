// Package rewrite converts plain <img> tags to the cavimg <cav-img> web component
// and produces framework-specific registration guidance.
package rewrite

import (
	"path/filepath"
	"regexp"
	"strings"
)

// imgTag matches a single <img …> tag (with or without a trailing slash),
// capturing its attributes. It never matches <cav-img> or <image>.
var imgTag = regexp.MustCompile(`(?is)<img\b([^>]*?)\s*/?>`)

// Transform rewrites every <img> in content to <cav-img>, preserving attributes.
// It returns the new content and whether anything changed. Running it again on
// its own output produces no further change (idempotent).
func Transform(filename, content string) (string, bool) {
	jsx := isJSX(filename)
	changed := false
	out := imgTag.ReplaceAllStringFunc(content, func(m string) string {
		changed = true
		sub := imgTag.FindStringSubmatch(m)
		attrs := strings.TrimSpace(sub[1])
		return buildTag(attrs, jsx)
	})
	return out, changed
}

func isJSX(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".jsx" || ext == ".tsx"
}

func buildTag(attrs string, jsx bool) string {
	if jsx {
		if attrs == "" {
			return "<cav-img />"
		}
		return "<cav-img " + attrs + " />"
	}
	if attrs == "" {
		return "<cav-img></cav-img>"
	}
	return "<cav-img " + attrs + "></cav-img>"
}

// Wiring returns registration-guidance steps for a framework and whether the
// wiring must be done manually (true for frameworks without a fully specified path).
func Wiring(framework string) ([]string, bool) {
	switch framework {
	case "Plain HTML":
		return []string{
			"Register <cav-img> with a module script in your HTML:",
			`<script type="module">import 'cavimg'</script>`,
			`Or via CDN: <script src="https://cdn.jsdelivr.net/npm/cavimg"></script>`,
		}, false
	case "Vite", "Vite+React":
		return []string{
			"Add a side-effect import in your entry module (e.g. src/main.ts or src/main.tsx):",
			"import 'cavimg';",
			"This auto-registers <cav-img>; no useEffect is needed for a client-only SPA.",
		}, false
	case "Next.js":
		return []string{
			"Web Components are client-only. In a 'use client' component, register on mount:",
			"useEffect(() => { import('cavimg').then(m => m.defineCavImg()); }, []);",
		}, false
	case "Angular":
		return []string{
			"Add CUSTOM_ELEMENTS_SCHEMA to the component and register the element:",
			"import { CUSTOM_ELEMENTS_SCHEMA } from '@angular/core';",
			"import { defineCavImg } from 'cavimg';",
			"// @Component({ ..., schemas: [CUSTOM_ELEMENTS_SCHEMA] })",
			"// in the constructor: defineCavImg();",
		}, false
	default:
		return []string{
			"Manual wiring required for " + framework +
				". See https://github.com/TheeraphatStudent/cavimg for framework guidance.",
		}, true
	}
}
