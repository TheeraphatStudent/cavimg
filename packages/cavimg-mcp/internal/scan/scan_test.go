package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestScanFindsImgTagsAndImports(t *testing.T) {
	dir := write(t, map[string]string{
		"index.html":        "<h1>x</h1>\n<img src=\"a.png\" alt=\"a\">\n",
		"src/App.tsx":       "import hero from './hero.png';\nexport const A = () => <img src={hero} />;\n",
		"node_modules/x.js": "<img src='ignore'>",
	})
	hits, err := Run(dir, "")
	if err != nil {
		t.Fatal(err)
	}
	var tags, imports int
	for _, h := range hits {
		if h.Kind == "img-tag" {
			tags++
		}
		if h.Kind == "image-import" {
			imports++
		}
		if h.File == "node_modules/x.js" {
			t.Errorf("node_modules must be skipped, got hit in %s", h.File)
		}
	}
	if tags != 2 {
		t.Errorf("img-tag hits = %d, want 2", tags)
	}
	if imports != 1 {
		t.Errorf("image-import hits = %d, want 1", imports)
	}
}

func TestScanReportsLineAndRelPath(t *testing.T) {
	dir := write(t, map[string]string{
		"page.html": "line1\nline2\n<img src=\"x.png\">\n",
	})
	hits, _ := Run(dir, "")
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].File != "page.html" {
		t.Errorf("file = %q, want page.html", hits[0].File)
	}
	if hits[0].Line != 3 {
		t.Errorf("line = %d, want 3", hits[0].Line)
	}
}
