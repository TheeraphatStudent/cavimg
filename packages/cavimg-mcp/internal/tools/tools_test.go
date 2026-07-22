package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func setupWorkspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CAVIMG_WORKSPACE_ROOT", root)
	return root
}

func TestDetectHandler(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json":  `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/tsconfig.json": "{}",
		"proj/src/main.tsx":  "",
	})
	_, out, err := DetectHandler(context.Background(), nil, DetectInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Framework != "Vite+React" {
		t.Errorf("framework = %q, want Vite+React", out.Framework)
	}
}

func TestDetectHandlerRejectsEscape(t *testing.T) {
	setupWorkspace(t, map[string]string{"proj/package.json": "{}"})
	if _, _, err := DetectHandler(context.Background(), nil, DetectInput{ProjectPath: "proj/../.."}); err == nil {
		t.Fatal("expected error for escaping path")
	}
}

func TestInstallHandlerDryRunDoesNotExecute(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json":   `{"devDependencies":{"vite":"^5"}}`,
		"proj/pnpm-lock.yaml": "",
	})
	_, out, err := InstallHandler(context.Background(), nil, InstallInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Command != "pnpm add cavimg" {
		t.Errorf("command = %q, want 'pnpm add cavimg'", out.Command)
	}
	if out.Executed {
		t.Error("dry-run must not execute")
	}
	if out.ExitCode != nil {
		t.Error("dry-run exit code must be nil")
	}
}

func TestApplyDryRunReturnsDiffWithoutWriting(t *testing.T) {
	root := setupWorkspace(t, map[string]string{
		"proj/package.json": `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/src/App.tsx":  "export default () => <img src=\"/a.png\" alt=\"a\" />;\n",
	})
	before, _ := os.ReadFile(filepath.Join(root, "proj", "src", "App.tsx"))
	_, out, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out.Diff == "" {
		t.Error("expected a non-empty diff on dry-run")
	}
	after, _ := os.ReadFile(filepath.Join(root, "proj", "src", "App.tsx"))
	if string(before) != string(after) {
		t.Error("dry-run must not modify files")
	}
}

func TestApplyIsIdempotent(t *testing.T) {
	setupWorkspace(t, map[string]string{
		"proj/package.json": `{"dependencies":{"react":"^18"},"devDependencies":{"vite":"^5"}}`,
		"proj/src/App.tsx":  "export default () => <img src=\"/a.png\" alt=\"a\" />;\n",
	})
	no := false
	_, out1, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj", DryRun: &no})
	if err != nil {
		t.Fatal(err)
	}
	if len(out1.ChangedFiles) == 0 {
		t.Fatal("first apply should change files")
	}
	_, out2, err := ApplyHandler(context.Background(), nil, ApplyInput{ProjectPath: "proj"})
	if err != nil {
		t.Fatal(err)
	}
	if out2.Diff != "" || len(out2.ChangedFiles) != 0 {
		t.Fatalf("second apply should be empty; diff=%q files=%v", out2.Diff, out2.ChangedFiles)
	}
}
