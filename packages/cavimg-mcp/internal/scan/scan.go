// Package scan finds image usages (<img> tags and image imports) in a project.
package scan

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Hit is a single image usage. File is relative to the scanned dir (forward slashes).
type Hit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Kind  string `json:"kind"` // "img-tag" | "image-import"
	Match string `json:"match"`
}

var (
	imgTagRe    = regexp.MustCompile(`(?i)<img\b`)
	imgImportRe = regexp.MustCompile(`(?i)\bimport\b[^\n]*['"][^'"\n]*\.(?:png|jpe?g|gif|webp|avif|svg)['"]`)
	defaultExts = map[string]bool{
		".html": true, ".htm": true, ".jsx": true, ".tsx": true,
		".vue": true, ".svelte": true, ".astro": true,
	}
	skipDirs = map[string]bool{
		"node_modules": true, "dist": true, ".next": true, "build": true, ".git": true,
	}
)

// Run scans dir for image usages. A non-empty glob (filepath.Match on the base
// name) overrides the default extension filter.
func Run(dir, glob string) ([]Hit, error) {
	var hits []Hit
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !selected(d.Name(), glob) {
			return nil
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return nil
		}
		fileHits, scanErr := scanFile(path, filepath.ToSlash(rel))
		if scanErr != nil {
			return nil
		}
		hits = append(hits, fileHits...)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
	return hits, nil
}

func selected(base, glob string) bool {
	if glob != "" {
		ok, _ := filepath.Match(glob, base)
		return ok
	}
	return defaultExts[strings.ToLower(filepath.Ext(base))]
}

func scanFile(path, rel string) ([]Hit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hits []Hit
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	line := 0
	for sc.Scan() {
		line++
		text := sc.Text()
		if loc := imgTagRe.FindStringIndex(text); loc != nil {
			hits = append(hits, Hit{File: rel, Line: line, Kind: "img-tag", Match: trim(text)})
			continue
		}
		if imgImportRe.MatchString(text) {
			hits = append(hits, Hit{File: rel, Line: line, Kind: "image-import", Match: trim(text)})
		}
	}
	return hits, sc.Err()
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 200 {
		return s[:200]
	}
	return s
}
