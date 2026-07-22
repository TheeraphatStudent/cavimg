// Package tools implements the four cavimg-mcp tool handlers and registers them
// on an MCP server. Every handler confines its project_path to the workspace root.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"cavimg-mcp/internal/detect"
	"cavimg-mcp/internal/rewrite"
	"cavimg-mcp/internal/scan"
	"cavimg-mcp/internal/textdiff"
	"cavimg-mcp/internal/workspace"
)

const maxOutput = 8192

// ---- shared ----

func confine(projectPath string) (string, error) {
	root, err := workspace.Root()
	if err != nil {
		return "", fmt.Errorf("workspace root unavailable: %w", err)
	}
	p, err := workspace.Confine(root, projectPath)
	if err != nil {
		return "", fmt.Errorf("project_path rejected: %w", err)
	}
	return p, nil
}

func text(summary string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
}

// ---- detect_project ----

type DetectInput struct {
	ProjectPath string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
}

type DetectOutput struct {
	PackageManager string            `json:"package_manager"`
	Framework      string            `json:"framework"`
	TypeScript     bool              `json:"typescript"`
	SourceDirs     []string          `json:"source_dirs"`
	Evidence       map[string]string `json:"evidence"`
	Summary        string            `json:"summary"`
}

func DetectHandler(ctx context.Context, req *mcp.CallToolRequest, in DetectInput) (*mcp.CallToolResult, DetectOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, DetectOutput{}, err
	}
	r, err := detect.Run(dir)
	if err != nil {
		return nil, DetectOutput{}, err
	}
	out := DetectOutput{
		PackageManager: r.PackageManager,
		Framework:      r.Framework,
		TypeScript:     r.TypeScript,
		SourceDirs:     r.SourceDirs,
		Evidence:       r.Evidence,
	}
	ts := "no TypeScript"
	if out.TypeScript {
		ts = "TypeScript"
	}
	out.Summary = fmt.Sprintf("%s project using %s (%s); source dirs: %s",
		out.Framework, out.PackageManager, ts, strings.Join(out.SourceDirs, ", "))
	return text(out.Summary), out, nil
}

// ---- install_cavimg ----

type InstallInput struct {
	ProjectPath    string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Version        string `json:"version,omitempty" jsonschema:"cavimg version (e.g. 1.0.1); defaults to latest"`
	PackageManager string `json:"package_manager,omitempty" jsonschema:"npm|pnpm|yarn|bun; auto-detected if omitted"`
	DryRun         *bool  `json:"dry_run,omitempty" jsonschema:"if true (default) report the command without running it"`
}

type InstallOutput struct {
	Command   string `json:"command"`
	Executed  bool   `json:"executed"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	Truncated bool   `json:"truncated"`
	Summary   string `json:"summary"`
}

func InstallHandler(ctx context.Context, req *mcp.CallToolRequest, in InstallInput) (*mcp.CallToolResult, InstallOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, InstallOutput{}, err
	}
	pm := in.PackageManager
	if pm == "" {
		r, derr := detect.Run(dir)
		if derr != nil {
			return nil, InstallOutput{}, derr
		}
		pm = r.PackageManager
	}
	name, args := installCmd(pm, in.Version)
	cmdStr := name + " " + strings.Join(args, " ")

	dry := in.DryRun == nil || *in.DryRun
	if dry {
		out := InstallOutput{
			Command:  cmdStr,
			Executed: false,
			Summary:  "dry-run: would run `" + cmdStr + "` in " + in.ProjectPath,
		}
		return text(out.Summary), out, nil
	}

	if _, lookErr := exec.LookPath(name); lookErr != nil {
		out := InstallOutput{
			Command:  cmdStr,
			Executed: false,
			Stderr:   name + " is not available in this environment",
			Summary:  name + " not found; nothing was installed",
		}
		return text(out.Summary), out, nil
	}

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var so, se bytes.Buffer
	cmd.Stdout, cmd.Stderr = &so, &se
	runErr := cmd.Run()

	code := 0
	if runErr != nil {
		if ee, ok := runErr.(*exec.ExitError); ok {
			code = ee.ExitCode()
		} else {
			code = -1
			se.WriteString("\n" + runErr.Error())
		}
	}
	stdout, tr1 := truncate(so.String())
	stderr, tr2 := truncate(se.String())
	out := InstallOutput{
		Command:   cmdStr,
		Executed:  true,
		ExitCode:  &code,
		Stdout:    stdout,
		Stderr:    stderr,
		Truncated: tr1 || tr2,
		Summary:   fmt.Sprintf("ran `%s` (exit %d)", cmdStr, code),
	}
	return text(out.Summary), out, nil
}

func installCmd(pm, version string) (string, []string) {
	spec := "cavimg"
	if version != "" {
		spec = "cavimg@" + version
	}
	switch pm {
	case "pnpm":
		return "pnpm", []string{"add", spec}
	case "yarn":
		return "yarn", []string{"add", spec}
	case "bun":
		return "bun", []string{"add", spec}
	default:
		return "npm", []string{"install", spec}
	}
}

func truncate(s string) (string, bool) {
	if len(s) > maxOutput {
		return s[:maxOutput], true
	}
	return s, false
}

// ---- list_image_usages ----

type ListInput struct {
	ProjectPath string `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Glob        string `json:"glob,omitempty" jsonschema:"optional filename glob (filepath.Match on base names)"`
}

type ListOutput struct {
	Hits    []scan.Hit `json:"hits"`
	Count   int        `json:"count"`
	Summary string     `json:"summary"`
}

func ListHandler(ctx context.Context, req *mcp.CallToolRequest, in ListInput) (*mcp.CallToolResult, ListOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, ListOutput{}, err
	}
	hits, err := scan.Run(dir, in.Glob)
	if err != nil {
		return nil, ListOutput{}, err
	}
	if hits == nil {
		hits = []scan.Hit{}
	}
	out := ListOutput{
		Hits:    hits,
		Count:   len(hits),
		Summary: fmt.Sprintf("found %d image usage(s)", len(hits)),
	}
	return text(out.Summary), out, nil
}

// ---- apply_cavimg ----

type ApplyInput struct {
	ProjectPath string   `json:"project_path" jsonschema:"path to the project root, within the workspace"`
	Files       []string `json:"files,omitempty" jsonschema:"specific files to rewrite; defaults to all detected"`
	DryRun      *bool    `json:"dry_run,omitempty" jsonschema:"if true (default) return a diff and change nothing on disk"`
}

type WiringInfo struct {
	Framework string   `json:"framework"`
	Steps     []string `json:"steps"`
	Manual    bool     `json:"manual"`
}

type ApplyOutput struct {
	DryRun       bool       `json:"dry_run"`
	Diff         string     `json:"diff"`
	ChangedFiles []string   `json:"changed_files"`
	Hunks        int        `json:"hunks"`
	Wiring       WiringInfo `json:"wiring"`
	Summary      string     `json:"summary"`
}

func ApplyHandler(ctx context.Context, req *mcp.CallToolRequest, in ApplyInput) (*mcp.CallToolResult, ApplyOutput, error) {
	dir, err := confine(in.ProjectPath)
	if err != nil {
		return nil, ApplyOutput{}, err
	}
	det, err := detect.Run(dir)
	if err != nil {
		return nil, ApplyOutput{}, err
	}

	files := in.Files
	if len(files) == 0 {
		hits, serr := scan.Run(dir, "")
		if serr != nil {
			return nil, ApplyOutput{}, serr
		}
		seen := map[string]bool{}
		for _, h := range hits {
			if h.Kind == "img-tag" && !seen[h.File] {
				seen[h.File] = true
				files = append(files, h.File)
			}
		}
		sort.Strings(files)
	}

	dry := in.DryRun == nil || *in.DryRun
	var diffs []string
	changed := []string{}
	hunks := 0
	for _, f := range files {
		abs, cerr := workspace.Confine(dir, f)
		if cerr != nil {
			return nil, ApplyOutput{}, fmt.Errorf("file rejected: %w", cerr)
		}
		content, rerr := os.ReadFile(abs)
		if rerr != nil {
			continue
		}
		newContent, didChange := rewrite.Transform(abs, string(content))
		if !didChange {
			continue
		}
		rel, _ := filepath.Rel(dir, abs)
		relSlash := filepath.ToSlash(rel)
		d := textdiff.Unified(relSlash, string(content), newContent)
		if d == "" {
			continue
		}
		diffs = append(diffs, d)
		hunks += strings.Count(d, "@@ -")
		changed = append(changed, relSlash)
		if !dry {
			if werr := os.WriteFile(abs, []byte(newContent), 0o644); werr != nil {
				return nil, ApplyOutput{}, werr
			}
		}
	}

	steps, manual := rewrite.Wiring(det.Framework)
	verb := "would rewrite"
	if !dry {
		verb = "rewrote"
	}
	out := ApplyOutput{
		DryRun:       dry,
		Diff:         strings.Join(diffs, "\n"),
		ChangedFiles: changed,
		Hunks:        hunks,
		Wiring:       WiringInfo{Framework: det.Framework, Steps: steps, Manual: manual},
		Summary:      fmt.Sprintf("%s %d file(s) to <cav-img>; see wiring for %s registration", verb, len(changed), det.Framework),
	}
	return text(out.Summary), out, nil
}

// ---- registration ----

// Register adds all four cavimg tools to the server.
func Register(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "detect_project",
		Description: "Detect a frontend project's package manager, framework, TypeScript usage, and source dirs.",
	}, DetectHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "install_cavimg",
		Description: "Install the cavimg npm package with the project's package manager. Defaults to dry_run.",
	}, InstallHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_image_usages",
		Description: "List <img> tags and image imports in a project.",
	}, ListHandler)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "apply_cavimg",
		Description: "Rewrite <img> tags to cavimg's <cav-img>. Defaults to dry_run (returns a diff, changes nothing).",
	}, ApplyHandler)
}
