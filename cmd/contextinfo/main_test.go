package main

import (
	"encoding/json"
	"io"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildCLI compiles the contextinfo binary into a temp dir and returns its path.
// It exercises the real executable end-to-end (flag parsing, output, exit codes),
// which the library unit tests don't cover.
func buildCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "contextinfo")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	out, err := exec.Command("go", "build", "-o", bin, ".").CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// runCLI runs the binary and returns its stdout and exit code.
func runCLI(t *testing.T, bin string, args ...string) (stdout string, code int) {
	t.Helper()
	var so strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Stdout = &so
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if ok := asExitError(err, &ee); ok {
			return so.String(), ee.ExitCode()
		}
		t.Fatalf("run %v: %v", args, err)
	}
	return so.String(), 0
}

func asExitError(err error, target **exec.ExitError) bool {
	if ee, ok := err.(*exec.ExitError); ok {
		*target = ee
		return true
	}
	return false
}

func TestCLIVersion(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--version")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if strings.TrimSpace(out) == "" {
		t.Error("--version printed nothing")
	}
}

func TestCLIDefaultIsEnvVar(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin) // no args
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "git_commit=") {
		t.Errorf("default output should be envvar lines, got:\n%s", out)
	}
	if strings.Contains(out, "{") {
		t.Errorf("default output should not be JSON:\n%s", out)
	}
}

func TestCLIEnvVarPrefix(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=envvar", "--prefix", "TF_VAR_")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "TF_VAR_git_commit=") {
		t.Errorf("envvar --prefix not applied:\n%s", out)
	}
	// Booleans are bare, strings are single-quoted.
	if !strings.Contains(out, "TF_VAR_git_dirty=") {
		t.Errorf("missing TF_VAR_git_dirty:\n%s", out)
	}
}

func TestCLIJSON(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=json")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(out), &info); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}
	for _, key := range []string{"ci", "git", "runtime"} {
		if _, ok := info[key]; !ok {
			t.Errorf("JSON missing %q key", key)
		}
	}
}

func TestCLIHelp(t *testing.T) {
	bin := buildCLI(t)
	// --help/-h prints usage and exits 0; flag sends it to stderr, so capture both.
	out, err := exec.Command(bin, "--help").CombinedOutput()
	if err != nil {
		t.Fatalf("--help exited non-zero: %v\n%s", err, out)
	}
	for _, want := range []string{"Usage", "Formats", "Examples", "envvar", "-format", "-prefix", "TF_VAR_"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("--help output missing %q\n%s", want, out)
		}
	}
}

func TestCLIText(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=text")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	for _, want := range []string{"ci.name", "git.commit", "runtime.os"} {
		if !strings.Contains(out, want) {
			t.Errorf("text output missing %q\n%s", want, out)
		}
	}
}

func TestCLIJSONFlat(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=json-flat")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("json-flat is not valid JSON: %v\n%s", err, out)
	}
	if _, ok := m["git_commit"]; !ok {
		t.Errorf("json-flat missing flat key git_commit\n%s", out)
	}
	if _, nested := m["git"]; nested {
		t.Errorf("json-flat should be flat, found nested \"git\"\n%s", out)
	}
}

func TestCLITFVars(t *testing.T) {
	bin := buildCLI(t)

	// Default: no prefix.
	hcl, code := runCLI(t, bin, "--format=tfvars")
	if code != 0 {
		t.Fatalf("tfvars: exit = %d, want 0", code)
	}
	if !strings.Contains(hcl, "runtime_os") || !strings.Contains(hcl, " = ") {
		t.Errorf("tfvars (HCL) output missing expected variable assignment\n%s", hcl)
	}
	if strings.Contains(hcl, "contextinfo_") {
		t.Errorf("tfvars should have no prefix by default\n%s", hcl)
	}

	js, code := runCLI(t, bin, "--format=tfvars-json")
	if code != 0 {
		t.Fatalf("tfvars-json: exit = %d, want 0", code)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(js), &m); err != nil {
		t.Fatalf("tfvars-json is not valid JSON: %v\n%s", err, js)
	}
	if _, ok := m["runtime_os"]; !ok {
		t.Errorf("tfvars-json missing runtime_os\n%s", js)
	}
}

func TestCLIPrefix(t *testing.T) {
	bin := buildCLI(t)
	out, code := runCLI(t, bin, "--format=tfvars", "--prefix", "TF_VAR_")
	if code != 0 {
		t.Fatalf("exit = %d, want 0", code)
	}
	if !strings.Contains(out, "TF_VAR_git_commit") {
		t.Errorf("--prefix not applied; expected TF_VAR_git_commit\n%s", out)
	}
}

func TestCLIUnknownFormatFails(t *testing.T) {
	bin := buildCLI(t)
	_, code := runCLI(t, bin, "--format=bogus")
	if code == 0 {
		t.Error("expected non-zero exit for an unknown format")
	}
}
